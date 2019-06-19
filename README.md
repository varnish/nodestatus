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

* ``--listen-host string``: Listen host (default "127.0.0.1")
* ``--listen-port int``: Listen port (default 8080)
* ``--interval int``: Number of seconds to use as interval for averages (default 1)
* ``--net-dev string``: Network interface to read stats from, examples are "eth0" or "bond0" or "all" to show all network interfaces combined (default "all")
* ``--net-threshold int``: Data gather interval in seconds, examples are "1000", "10 Kbps", "4.5 Gbps" and "0.3 Tbps" (default "800 Mbps")
* ``--maintenance string``: Path to a file in the file system which indicates maintenance mode (default /etc/varnish/maintenance)

Example output:

```
$ curl -i http://localhost:8080
HTTP/1.1 200 OK
Cache-Control: max-age=1, stale-while-revalidate=1
Content-Type: application/json
Date: Wed, 19 Jun 2019 11:44:15 GMT
Content-Length: 266

{
    "free": true,
    "reason": "Normal operation",
    "load1": 2.15,
    "load5": 1.83,
    "load15": 1.72,
    "net": "99 Mbps",
    "net-threshold": "1.0 Gbps",
    "net-utilization": 9,
    "time": 1560944655,
    "uptime": 2,
    "hostname": "work-2.local"
}
```

Explanation:

* ``free: true`` means that the node has available resources to handle more clients.
* The current transfer rate (99 Mbps) is at 9% (net-utilization) of the threshold (1 Gbps).

