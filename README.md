# filter-prometheus

## Description
This filter implements a Prometheus exporter to expose OpenSMTPD metrics.


## Features
The filter currently supports:

- smtp-in and smtp-out metrics


## Dependencies
The filter is written in Golang and doesn't have any dependencies beyond standard library.

It requires OpenSMTPD 6.7.0 or higher.


## How to install
Clone the repository, build and install the filter:
```
$ cd filter-prometheus/
$ go build
$ doas install -m 0555 filter-prometheus /usr/local/libexec/smtpd/filter-prometheus
```


## How to configure
The filter itself requires no configuration.

It must be declared in smtpd.conf and attached to a listener or relay action:
```
filter "prometheus" proc-exec "filter-prometheus"

listen on all filter "prometheus"

action "foobar" relay filter "prometheus"
```

The exporter will listen on `localhost:13742` by default.

The filter supports an `-exporter` parameter to provide an alternate interface:

```
filter "prometheus" proc-exec "filter-prometheus -exporter 192.168.1.27:43434"

listen on all filter "prometheus"

action "foobar" relay filter "prometheus"
```