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

