# NMP: Nagios Metrics Processor

NMP (Nagios Metrics Processor) is a simple metrics collector for use with Nagios.
It is designed to receive and process collectd metrics and send passive check results to Nagios.

## Install

```bash
go get -u github.com/milk/nmp/cmd/nmp
```

## Build for a specific OS and architecture

```bash
env GOOS=linux GOARCH=amd64 go build -v github.com/milk/nmp/cmd/nmp
```

## Configuration

```hcl
check_results_dir = "/usr/local/nagios/var/spool/checkresults"

check "memory" {
    plugin = "memory"
    comparator = "<="
    type_instance = "free"
    warning = "${1024 * 1024 * 1024}"
    critical = "${512 * 1024 * 1024}"
    
    host "db.*" {
        warning = "${0.2 * 15 * 1024 * 1024 * 1024}"
        critical = "${0.1 * 15 * 1024 * 1024 * 1024}"
    }
    
    host "(web.*|worker.*)" {
        warning = "${0.2 * 4 * 1024 * 1024 * 1024}"
        critical = "${0.1 * 4 * 1024 * 1024 * 1024}"
    }
}

check "load_shortterm" {
    plugin = "load"
    value = "{{ (index .Values 0) }}"
    warning = "0.7"
    critical = "0.8"
}

check "load_midterm" {
    plugin = "load"
    value = "{{ (index .Values 1) }}"
    warning = "0.7"
    critical = "0.8"
}

check "load_longterm" {
    plugin = "load"
    value = "{{ (index .Values 2) }}"
    warning = "0.7"
    critical = "0.8"
}

```
