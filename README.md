# minio-benchmarking

Simple benchmarking utility to test some of the core algorithms of minio object storage server.

## How to run

```
$ go test -timeout 1h -cpu 1,2,4,8,16,32,64 -bench .
```

NB Adjust the number of cores accordingly to the maximum number of cores in your server.
