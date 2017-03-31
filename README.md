# nodestatus

Lightweight web service with a built in web server to print node status.

## Installation

```
go get github.com/varnish/nodestatus
make deps
make build
```

``$GOPATH/bin`` has to be in path for ``make build`` to succeed.

## Running

Standard way of running it:

```
bin/nodestatus-darwin-amd64
```

Parameters:

* ``--interface-stats string``: Network interface to read stats from (default "all")
* ``--interval int``: Data gather interval in seconds (default 1)
* ``--listen-host string``: Listen host (default "127.0.0.1")
* ``--listen-port int``: Listen port (default 8080)

Example output:

```
$ curl -i http://localhost:8080/
HTTP/1.1 200 OK
Cache-Control: max-age=1
Content-Type: application/json
Date: Fri, 31 Mar 2017 13:40:26 GMT
Content-Length: 149

{
    "load1": 1.56,
    "load5": 2.09,
    "load15": 2.07,
    "net-bps-tx": 7421,
    "net-bps-rx": 8614,
    "time": 1490967626,
    "uptime": 5
}
```

By default it shows the network stats (bytes {in,out} per second) all network interfaces combined. Using the ``--interface-stats`` parameter it is possible to specify a specific interface, for example ``eth0`` or ``bond0``.

## Static content

A small gif is served from ``/static/r20.gif``, which can be used by Cedexis for measurements.
