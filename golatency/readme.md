# Configuration

Quick and dirty during early exploration development.

Environment variables:
```
GOLATENCY_ITERATIONS=100
GOLATENCY_JSONOUT=FALSE
GOLATENCY_REDISAUTHTOKEN=<auth>
GOLATENCY_REDISCONNECTIONSTRING=<host:port>
```

If GOLATENCY_JSONOUT is set to true, only the JSON output will be printed to
stdout.


If storing results in mongo: 
```
GOLATENCY_MONGOCOLLECTIONNAME=<name>
GOLATENCY_MONGOCONNSTRING=<connstring>
GOLATENCY_MONGODBNAME=<dbname>
GOLATENCY_MONGOUSERNAME=<username>
GOLATENCY_MONGOPASSWORD=<password>
```


# Results
The latency numbers are in nanoseconds, and represent the point of view of the
client. As such it includes networking.
