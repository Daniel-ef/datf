from test.utils.utils import SocketIO, go_thread, get_logger
import queue
import copy

logger = get_logger("Node")
Info = logger.info
Debug = logger.debug


class LamportMutexNode:
    def __init__(self, ports, wsPortAc, pid, ts):
        self.ports = ports
        self.wsPortAc = wsPortAc
        self.pid = pid
        self.ts = ts

        self.pqueue = queue.PriorityQueue()
        self.resp = 0
        self.nodes_conn = copy.deepcopy(self.ports)
        self.listner = None
        self.state = 0  # 0 - free, 1 - trying to enter, 2 - entered
        self.cmdConn = None
        self.wsConn = None

    def set_connections(self):
        self.listner = SocketIO(self.ports[self.pid], logger=Debug).set_inbound_connection()
        for i in range(len(self.ports)):
            if i == self.pid:
                self.nodes_conn[i] = None
            else:
                self.nodes_conn[i] = SocketIO(self.ports[i], logger=Debug).set_outbound_connection()
        self.wsConn = SocketIO(self.wsPortAc, logger=Debug).set_outbound_connection()
        self.wsConn.write_to_socket("Ready")

    def handle_requests(self):
        while True:
            req = self.listner.read_from_socket().split(",")
            req = req[0] if len(req) == 1 else req

            if req != "state":
                Info("Pid: {}. Get request: {}".format(self.pid, req))
            if req == "ok":
                self.resp += 1
            elif req == "release":
                self.pqueue.get()
            elif req == "tcs":
                go_thread(self.take_cs)
            elif req == "rcs":
                go_thread(self.release_cs)
            elif req == "state":
                self.wsConn.write_to_socket(",".join(map(str, [self.pid, self.state])))
            elif req[0] == "cs_req":
                self.ts = max(self.ts, int(req[1]) + 1)
                self.nodes_conn[int(req[2])].write_to_socket("ok")
                Info("Pid: {}. Sent ok".format(self.pid))
                self.pqueue.put([int(req[1]), int(req[2])])

    def take_cs(self):
        Info("Pid: {}. Trying to enter to cs.".format(self.pid))
        self.state = 1
        self.ts += 1
        req = [self.ts, self.pid]
        self.pqueue.put(req)
        for conn in self.nodes_conn:
            if conn:
                conn.write_to_socket("cs_req," + ",".join(list(map(str, req))))
                Info("Pid: {} Request sent. [ts: {}, pid: {}]".format(self.pid, req[0], req[1]))

        while self.resp != len(self.nodes_conn) - 1:
            continue
        self.resp = 0

        print("Pid: {}. {}".format(self.pid, self.pqueue.queue))
        while self.pqueue.queue[0] != req:
            continue
        Debug("Pid: {}. Entered to cs".format(self.pid))
        self.state = 2

    def release_cs(self):
        Debug("Pid: {}. Release cs.".format(self.pid))
        self.pqueue.get()
        self.ts += 1
        for conn in self.nodes_conn:
            if conn:
                conn.write_to_socket("release")
                Info("Pid: {}. Sent release to {}".format(self.pid, conn.port))
        self.state = 0

    def start(self):
        self.set_connections()

        go_thread(self.handle_requests, daemon=False)
