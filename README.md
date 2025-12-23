# statesaver: save and view terraform .tfstate

- save .tfstate
    - supports lock/unlock
    - keep history of all states
- view .tfstate
    - have WebUI

## boot server

local

- `mkdir data`
- `statesaver server -d data`

docker

- `mkdir data`
- `docker run -p 3000:3000 -v $PWD:/w -w /w ghcr.io/wtnb75/statesaver:main server -d data`

compose

```yaml
services:
  statesaver:
    image: ghcr.io/wtnb75/statesaver:main
    environment:
      STSV_DATADIR: /data
    volumes:
    - ./data:/data
    command:
    - server
    ports:
    - 3000:3000
```

## .tf example

```hcl2
terraform {
  backend "http" {
    # viewer: http://server.name:3000/html/
    address = "http://server.name:3000/api/state123"
    lock_address = "http://server.name:3000/api/state123"
    unlock_address = "http://server.name:3000/api/state123"
  }
}
```

## management commands

```
Usage:
  statesaver [OPTIONS] <command>

Application Options:
  -v, --verbose   DEBUG level
  -q, --quiet     WARNING level
  -d, --data-dir= data directory to store state [$STSV_DATADIR]

Help Options:
  -h, --help      Show this help message

Available commands:
  cat       cat files
  hcat      cat history
  history   list history
  ls        list files
  prune     prune history
  put       put files
  rollback  rollback to history
  server    boot webserver
```

### list all files

...
