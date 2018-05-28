# Lamport mutual exclusion algorithm
import random
import subprocess
import time

from test.Lamport.webSocketTests import websocket
from test.Lamport.lamportNode import LamportMutexNode
from test.utils.utils import go_thread


def make_ports_run(st_port, ws_port, stderr=False):
    # #################################################### Make ports for proxy
    ports_l = [[] for _ in range(threads_num)]
    for i in range(threads_num):
        for j in range(threads_num):
            ports_l[i].append(st_port)
            st_port += 1

    ports_str_l = ""
    for i in range(threads_num):
        for j in range(threads_num):
            if i != j:
                ports_str_l += "{} {} ".format(ports_l[i][j], ports_l[i][i])

    print("Ports between nodes:")
    for i in range(len(ports_l)):
        for j in range(len(ports_l[0])):
            if i != j:
                print("{} -> {} = {} -> {}".format(j, i, ports_l[i][j], ports_l[i][i]))

    ports_l = list(map(list, zip(*ports_l)))  # transpose self.nodes_conn

    # ################################################### Run hse-dss-efimov
    args = ["hse-dss-efimov", "channel"] + ports_str_l.strip().split() + [str(ws_port)]
    stderr = subprocess.DEVNULL if not stderr else None

    # ################################################### Make ports for commands

    return ports_l, subprocess.Popen(args, cwd="../..", stderr=stderr)


if __name__ == "__main__":
    threads_num = 4

    wsPort = 8082
    proxyPort = 10030
    ports, cmd = make_ports_run(proxyPort, wsPort, stderr=False)
    listners = [ports[i][i] for i in range(len(ports))]
    wsPortAc = listners[-1] + 1
    print("Port for websocket: {}".format(wsPortAc))

    ws_thread = go_thread(websocket, wsPort, wsPortAc, listners, {"type": "stress", "iters": 40}, daemon=False)

    time.sleep(1)
    for i in range(threads_num):
        go_thread(LamportMutexNode(ports[i],  wsPortAc, pid=i, ts=random.randint(0, 10),).start, daemon=True)

    ws_thread.join()
    exit(0)
