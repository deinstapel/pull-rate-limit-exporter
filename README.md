# pull-rate-limit-exporter

This exporter checks regulary the remaining pulls from `docker.io`-Registry, which is the standard registry of Docker.
They introduced a [Download rate limit](https://docs.docker.com/docker-hub/download-rate-limit/), which is quite limited and we ran into some problems with it. Therefore we decided to monitor those valies. 

The exporter does not use any available "pulls".

This exporter exposes two metrics:

- `dockerio_ratelimit_limit` - Maximum requests allowed within sliding window 
- `dockerio_ratelimit_remaining` - Remaining requests allowed within sliding window

## Configuration

Right now, there are not any configuration options.

## Usage

This exporter exposes its metrics at port 2342, which exposes an standard Prometheus endpoint.

```
➜  ~ curl http://pull-rate-limit-exporter:2342/metrics
# HELP dockerio_ratelimit_limit Allowed pulls
# TYPE dockerio_ratelimit_limit gauge
dockerio_ratelimit_limit 100
# HELP dockerio_ratelimit_remaining Remaining pulls
# TYPE dockerio_ratelimit_remaining gauge
dockerio_ratelimit_remaining 100
```

## Deployment

You need to deploy this exporter based on your environment. If you have multiple external IPs you need to deploy this once for each IP facing `docker.io`, e.g. as DaemonSet. If you have only one external IP, you can deploy this as a normal deployment with `replicas: 1`.

We also added an manifest for a ServiceMonitor, which is provided by the Prometheus operator. This can be used to add this to your Prometheus environment.

We included our example deployment within `kubernetes/resources.yaml`.

