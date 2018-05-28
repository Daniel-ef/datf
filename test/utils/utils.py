import struct
import socket
from threading import Thread
import json
import time
from collections import deque
import threading
from queue import Queue
import logging

INBOUND = 0
OUTBOUND = 1


def go_thread(func, *args, daemon=False):
    if not callable(func):
        raise ValueError(type(func), "is not callable")
    t = threading.Thread(target=func, args=args, daemon=daemon)
    t.start()
    return t


def get_logger(name):
    Debug = logging.getLogger(name)
    Debug.setLevel(logging.DEBUG)

    ch = logging.StreamHandler()
    formatter = logging.Formatter('%(asctime)s - %(levelname)s - %(name)s - %(message)s')
    ch.setFormatter(formatter)

    Debug.addHandler(ch)
    return Debug


class SocketIO:
    def __init__(self, port, host="localhost", logger=print):
        self.port = port
        self.host = host
        self.conn = None
        self.buf = b''
        self.logger = logger
        self.messages = Queue()
        self.conn_type = None

    def set_outbound_connection(self):
        while True:
            try:
                self.conn = socket.create_connection((self.host, self.port))
                break
            except ConnectionRefusedError:
                self.logger("Connection refused on %s to %s" % (self.host, self.port))
        self.logger("%s. Connection to %i created" % (self.host, self.port))
        self.conn_type = OUTBOUND
        return self

    def __read_from_sockets(self, conn):
        if self.conn_type != INBOUND:
            raise ConnectionError

        n = 0
        while True:
            try:
                self.buf += conn.recv(1024)
            except socket.timeout:
                return None
            if len(self.buf) >= 4 and n == 0:
                n = int.from_bytes(self.buf[:4], byteorder="big")
                self.buf = self.buf[4:]
            if n != 0 and len(self.buf) >= n:
                s = self.buf[:n].decode()
                self.buf = self.buf[n:]
                self.messages.put(s)
                n = 0

    def run_accepting_connections(self, sock, number, timeout):
        while number != 0:
            conn, addr = sock.accept()
            conn.settimeout(timeout)
            self.logger("%s. Port %i is listening from %s" % (self.host, self.port, addr))
            go_thread(self.__read_from_sockets, conn)
            number -= 1

    def set_inbound_connection(self, number=-1, timeout=None):
        self.conn_type = INBOUND
        sock = socket.socket()
        sock.bind(('', self.port))
        sock.listen()
        go_thread(self.run_accepting_connections, sock, number, timeout)
        return self

    def read_from_socket(self):
        el = self.messages.get()
        self.messages.task_done()
        return el

    def write_to_socket(self, payload):
        if self.conn_type != OUTBOUND:
            raise ConnectionError

        if isinstance(payload, str):
            payload = payload.encode("utf-8")
        assert isinstance(payload, bytes)
        header = struct.pack("!I", len(payload))
        Message = bytes(header) + payload
        n = 0
        while n < len(Message):
            r = self.conn.send(Message[n:])
            if r == 0:
                self.logger("connection was closed")
                return
            n += r

    def get_port(self):
        return self.port

    def close(self):
        self.conn.close()


class WebSocketIO:
    def __init__(self, port=8080, listners=None):
        self.ws = create_connection("ws://localhost:{}/ws".format(port))
        self.layer = dict(zip(map(str, listners), range(len(listners))))
        self.msgs = [deque() for _ in range(len(listners))]
        self.handle_flag = True
        Thread(target=self.handle_msgs).start()

    def handle_msgs(self):
        while self.handle_flag:
            msg = self.ws.recv()
            if not msg:
                continue
            msg = json.loads(msg)
            self.msgs[self.layer[msg["dst"]]].append(msg)

    def get_msgs(self):
        msgs = []
        for deq in self.msgs:
            msgs.extend(list(deq))
        return msgs

    def get_threads_with_msgs(self, non_blocking=False):
        threads = []
        while len(threads) == 0:
            for i, deq in enumerate(self.msgs):
                if len(deq) != 0:
                    threads.append(i)
            if non_blocking:
                break
        return threads

    def accept_msg(self, thread_num):
        if len(self.msgs[thread_num]) == 0:
            raise KeyError
        msg = self.msgs[thread_num].popleft()
        self.ws.send(json.dumps({"kind": 3, "msgNumber": msg["msgNumber"], "data": "1"}))
        time.sleep(0.1)

    def reject_msg(self, thread_num):
        if len(self.msgs[thread_num]) == 0:
            raise KeyError
        msg = self.msgs[thread_num].popleft()
        self.ws.send(json.dumps({"kind": 3, "msgNumber": msg["msgNumber"], "data": "0"}))
        time.sleep(0.1)

    def close(self):
        self.handle_flag = False
        self.ws.close()
