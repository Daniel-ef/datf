import random
import time

from test.utils.utils import SocketIO, WebSocketIO, get_logger
from collections import Counter

logger = get_logger("WS")
Debug = logger.debug
Error = logger.error


def check_states(to_nodes_, from_nodes_, cur_states):
    for conn in to_nodes_:
        conn.write_to_socket("state")
        # Debug("State request sent to %s" % conn.get_port())
    for _ in range(len(to_nodes_)):
        ans = from_nodes_.read_from_socket().split(",")
        # Debug("State of pid {} is {}".format(*ans))
        cur_states[int(ans[0])] = int(ans[1])


def stress_test(to_nodes_, from_nodes_, ws_, iters_):
    global_counters = Counter()
    cur_states = [0] * len(to_nodes_)

    for conn in to_nodes_:
        conn.write_to_socket("tcs")
        global_counters["tcs"] += 1
        Debug("Cs cmd sent to %s" % conn.get_port())

    while True:
        ready_threads = ws_.get_threads_with_msgs(non_blocking=True)
        if len(ready_threads) == 0:
            if global_counters["rcs"] == iters_:
                break
            else:
                continue

        ready_th = random.choice(ready_threads)
        ws_.accept_msg(ready_th)
        time.sleep(0.1)  # allow internal work

        # Check states
        check_states(to_nodes_, from_nodes_, cur_states)

        # safety check
        assert Counter(cur_states)[2] <= 1

        # make progress in system
        for i in range(len(to_nodes_)):
            if cur_states[i] == 0:
                if global_counters["tcs"] < iters_:
                    to_nodes_[i].write_to_socket("tcs")
                    global_counters["tcs"] += 1
            elif cur_states[i] == 2:
                to_nodes_[i].write_to_socket("rcs")
                global_counters["rcs"] += 1
        global_counters["msgs"] += 1

    time.sleep(1)
    check_states(to_nodes_, from_nodes_, cur_states)

    assert global_counters["tcs"] == global_counters["rcs"] == iters_
    print(cur_states)
    assert cur_states == [0] * len(cur_states)

    print("Stress test passed")


def achive_state_(cur_states, summa, ach_states, ws, to_nodes, from_nodes):
    if summa != -1:
        cond = lambda x: sum(x) != summa
    else:
        cond = lambda x: Counter(x) == Counter(ach_states)

    while cond(cur_states):
        ready_threads = ws.get_threads_with_msgs()
        for ready_th in ready_threads:
            ws.accept_msg(ready_th)

        # TODO: is it normal?!
        time.sleep(1)

        # Check states
        for conn in to_nodes:
            conn.write_to_socket("state")
            # Debug("State request sent to %s" % conn.get_port())
        for i in range(len(to_nodes)):
            ans = from_nodes.read_from_socket().split(",")
            # Debug("State of pid {} is {}".format(*ans))
            cur_states[int(ans[0])] = int(ans[1])
        if sum(cur_states) > len(to_nodes) + 1:
            Error("Fatal: algorithm is wrong")
            exit(1)
    Debug("States: {}".format(cur_states))
    return cur_states


def websocket(ws_port, port_from_nodes, _listners, test):
    ws = WebSocketIO(ws_port, _listners)
    from_nodes = SocketIO(port_from_nodes, logger=Debug).set_inbound_connection()
    to_nodes = list(map(lambda port: SocketIO(port, logger=Debug).set_outbound_connection(), _listners))

    achive_state = lambda cur_state, summa, ach_state: achive_state_(cur_state, summa, ach_state, ws, to_nodes, from_nodes)

    # Waiting for nodes readiness
    for i in range(len(to_nodes)):
        from_nodes.read_from_socket()

    if test["type"] == "all":
        accept_all_test(to_nodes, achive_state)
    elif test["type"] == "stress":
        stress_test(to_nodes, from_nodes, ws, test["iters"])
    else:
        Error("There is no such command")


def accept_all_test(to_nodes, achive_state):
    states = [0] * len(to_nodes)

    # Send to everyone take critical section
    for conn in to_nodes:
        conn.write_to_socket("tcs")
        Debug("Cs cmd sent to %s" % conn.get_port())

    # Wating while one node entering cs
    states = achive_state(states, len(to_nodes) + 1, [])

    # Send to node release critical section
    for i, state in enumerate(states):
        if state == 2:
            to_nodes[i].write_to_socket("rcs")
            break

    # Wating while one node not enter cs
    states = achive_state(states, len(to_nodes), [])

    for i, state in enumerate(states):
        if state == 2:
            to_nodes[i].write_to_socket("rcs")
            break

    achive_state(states, 0, [])

    print("Test succeeded")
    exit(0)


