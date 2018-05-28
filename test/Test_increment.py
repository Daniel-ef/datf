import atexit
import subprocess
import threading

from test.utils.utils import SocketIO, WebSocketIO


def client(port_accept, port_send):

    conn_outbound = SocketIO(port_send).set_outbound_connection()

    conn_inbound = SocketIO(port_accept).set_inbound_connection(timeout=3)

    for i in range(3):
        conn_outbound.write_to_socket("1")
        print("Client. Send 1")

    receive_mes_num = 0
    while True:
        data = conn_inbound.read_from_socket()
        if not data:
            break
        print("Client. Receive: {}".format(data))
        receive_mes_num = data
    conn_inbound.close()
    conn_outbound.close()
    return receive_mes_num


def server(port_accept, port_send):
    conn_inbound = SocketIO(port_accept).set_inbound_connection()

    conn_outbound = SocketIO(port_send).set_outbound_connection()

    def close_con():
        conn_outbound.close()
        conn_inbound.close()

    atexit.register(close_con)

    count = 0
    while True:
        data = conn_inbound.read_from_socket()

        count += int(data)

        conn_outbound.write_to_socket(str(count))
        # print("Server. {} sent".format(count))


def websocket(mes_num, wsport):
    ws = WebSocketIO(wsport)

    while len(ws.get_msgs()) < mes_num:
        continue
    msgs = ws.get_msgs()
    for i in range(mes_num):
        ws.accept_msg(msgs[i]["msgNumber"])
    ws.close()


def test_increment(accept_mes_num=3):
    port = 15000
    wsport = 8081
    cmd = subprocess.Popen(["hse-dss-efimov", "channel", str(port), str(port+1), str(wsport)])

    threading.Thread(target=server, args=(port+1, port+2), daemon=True).start()
    threading.Thread(target=websocket, args=(accept_mes_num,wsport,)).start()
    received_mes_num = client(port+2, port)
    assert(received_mes_num == str(accept_mes_num))

    cmd.kill()


if __name__ == "__main__":
    test_increment(3)

