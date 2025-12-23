# statesaver: save and view terraform .tfstate

- save .tfstate
    - supports lock/unlock
    - keep history of all states
- view .tfstate
    - built-in simple WebUI

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

```
# statesaver ls
2025-12-23T22:59:21+09:00   1420 /state123
```

### cat files

```
# statesaver cat /state123
{
  "version": 4,
  "terraform_version": "1.5.7",
  "serial": 5,
  "lineage": "27074632-8326-ecfb-b44c-84addb04459f",
  "outputs": {},
  "resources": [
    {
      "mode": "managed",
      "type": "local_file",
  :
```

### put files

```
# statesaver put -p hello/ test.json test2.json
# statesaver ls
2025-12-23T23:26:40+09:00     18 /hello/test.json
2025-12-23T23:26:40+09:00     29 /hello/test2.json
2025-12-23T22:58:58+09:00    180 /state123
```

### list history

```
# statesaver history /state123
/state123
2025-12-23T22:59:21+09:00   1420 1h0ussqgcphmg (current)
2025-12-23T22:58:58+09:00    180 1h0uss4nr6qhg
2025-12-23T22:55:19+09:00   1420 1h0uslomptqi0
2025-12-23T22:55:17+09:00    180 1h0uslmdr8r20
2025-12-23T22:55:13+09:00   1420 1h0usljgo2sh8
```

### cat history

```
# statesaver hcat -f /state123 1h0uslmdr8r20

  "version": 4,
  "terraform_version": "1.5.7",
  "serial": 2,
  "lineage": "27074632-8326-ecfb-b44c-84addb04459f",
  "outputs": {},
  :
```

### prune history

```
# statesaver history /state123
/state123
2025-12-23T22:59:21+09:00   1420 1h0ussqgcphmg (current)
2025-12-23T22:58:58+09:00    180 1h0uss4nr6qhg
2025-12-23T22:55:19+09:00   1420 1h0uslomptqi0
2025-12-23T22:55:17+09:00    180 1h0uslmdr8r20
2025-12-23T22:55:13+09:00   1420 1h0usljgo2sh8
# statesaver prune /state123 --keep 3
/state123
{"time":"2025-12-23T23:17:23.104984+09:00","level":"INFO","msg":"removing","name":"/state123","history":"1h0usljgo2sh8","dry":false,"path":"state123/1h0usljgo2sh8"}
{"time":"2025-12-23T23:17:50.991316+09:00","level":"INFO","msg":"removing","name":"/state123","history":"1h0uslmdr8r20","dry":false,"path":"state123/1h0uslmdr8r20"}
# statesaver history /state123
/state123
2025-12-23T22:59:21+09:00   1420 1h0ussqgcphmg (current)
2025-12-23T22:58:58+09:00    180 1h0uss4nr6qhg
2025-12-23T22:55:19+09:00   1420 1h0uslomptqi0
```

### prune all files in tree

```
# statesaver prune --keep 3 --all
  :
```

### rollback to history

```
# statesaver history /state123
/state123
2025-12-23T22:59:21+09:00   1420 1h0ussqgcphmg (current)
2025-12-23T22:58:58+09:00    180 1h0uss4nr6qhg
2025-12-23T22:55:19+09:00   1420 1h0uslomptqi0
# statesaver cat /state123
{
  "version": 4,
  "terraform_version": "1.5.7",
  "serial": 5,
  :
# statesaver rollback -f /state123 -t 1h0uss4nr6qhg
# statesaver history /state123
/state123
2025-12-23T22:59:21+09:00   1420 1h0ussqgcphmg
2025-12-23T22:58:58+09:00    180 1h0uss4nr6qhg (current)
2025-12-23T22:55:19+09:00   1420 1h0uslomptqi0
# statesaver cat /state123
{
  "version": 4,
  "terraform_version": "1.5.7",
  "serial": 4,
  :
```

### edit file

```
# statesaver edit /state123
(edit file in your editor)
```
