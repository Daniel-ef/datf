$(document).ready(function() {
    var t = $('#table').DataTable();
    var socket = new WebSocket("ws://localhost:8080/ws");
    var counter = t.rows()[0].length;
    var nums_list = [];

    // Handle Connection ___________________________________________________

    socket.onopen = function() {
        $("#status").text("Соединение установлено");
        socket.send(JSON.stringify({
            kind: 2,
            request: "db"
        }));
    };

    socket.onclose = function(event) {
        if (event.wasClean) {
            alert('Соединение закрыто чисто');
            $("#status").text("Соединение закрыто чисто.");
        } else {
            $("#status").text('Обрыв соединения.'); // например, "убит" процесс сервера
        }
        $("#status").text($("#status").text() + ' Код: ' + event.code + ' причина: ' + event.reason);
    };

    socket.onerror = function(error) {
        $("#status").text("Ошибка " + error.message);
    };

    // Getting message _____________________________________________________

    socket.onmessage = function(event) {
        var data = JSON.parse(event.data);
        if (nums_list.indexOf(data.msgNumber) !== -1) {
            return;
        }

        t.row.add([
            data.msgNumber,
            data.src,
            data.dst,
            data.payload,
            '<button id="accept" type="button" class="btn btn-success">Accept</button>' +
            '<button id="denied" type="button" class="btn btn-danger">Reject</button>'
        ]).node().id = counter;
        counter += 1;
        nums_list.push(data.msgNumber);
        t.draw(false);
    };

    function send_response(that, decision) {
        var row_ind = parseInt(that.parentElement.parentElement.id);
        var t = $('#table').DataTable();
        var trow = t.row(row_ind).data();
        var msgNumber = trow[0];
        socket.send(JSON.stringify({
            kind: 3,
            msgNumber: msgNumber.toString(),
            data: decision.toString()
        }));
        if (decision) {
            trow[4] = '<div class="alert alert-success" role="alert"> Message accepted </div>';
        } else {
            trow[4] = '<div class="alert alert-danger" role="alert"> Message rejected </div>';
        }
        t.row(row_ind).data(trow).invalidate()
    }

    $(document).on("click", "#accept", function () {
        send_response(this, 1)
    });

    $(document).on("click", "#denied", function() {
        send_response(this, 0)
    });

});

