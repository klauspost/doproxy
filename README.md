# doproxy
Reverse Proxy load balancer for managing multiple DigitalOcean backends.

# Do not use: Still in development

This is an extension of the idea behind [doproxy](https://github.com/thisismitch/doproxy) by Mitchell Anicas. Instead of managing a reverse proxy (HAProxy), this *is* a revserse proxy.

## features
* Simple reverse proxy setup.
* Health checks on backends.
* Hot configuration reload.
* Selectable load balancing algorithm.
 
## todo 
* Tests
* Documentation
* Automatic droplet creation/destruction. 
* Make stats available
* Configurable error handling
