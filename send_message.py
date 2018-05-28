from test.utils.utils import SocketIO


if __name__ == "__main__":
    port = 10000
    conn = SocketIO(port).set_outbound_connection()
    conn.write_to_socket("hello")
    conn.write_to_socket("world")
