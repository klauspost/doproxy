# doproxy
Reverse Proxy load balancer for managing multiple DigitalOcean backends. It allows you to connect several backends to a single outgoing port, with load balancing between each server.

The server supports hot configuration reloading, so you can change settings and your backend inventory without having to restart the server. The server can automatically monitor your backends, and disable them if requests fail.

There is planned for automatic provisioning and de-provisioning, so your site can scale up and down depending on your demand.

[![Build Status](https://travis-ci.org/klauspost/doproxy.svg?branch=master)](https://travis-ci.org/klauspost/doproxy)
[![GoDoc][1]][2]

[1]: https://godoc.org/github.com/klauspost/doproxy/server?status.svg
[2]: https://godoc.org/github.com/klauspost/doproxy/server

# Warning: Alpha software

This software is still under development, and therefore not production ready. Perform your own tests before deploying to make sure it doesn't have any unintended side-effects.

This is an extension of the idea behind [doproxy](https://github.com/thisismitch/doproxy) by Mitchell Anicas. Instead of managing a reverse proxy (HAProxy), this *is* a revserse proxy.

## features
* Simple reverse proxy setup.
* Health checks on backends.
* Hot configuration reload.
* Selectable load balancing algorithm.
 
# setup


## todo 
* Load balancer tests
* Automatic droplet creation/destruction. 
* Make stats available
* Configurable error handling

# license
This software is released under the MIT Licence. See the LICENSE file for more deatails.

(c) 2015 Klaus Post
