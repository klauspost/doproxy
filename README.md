![doproxy](https://img.klauspost.com/doproxy-trans-700.png "doproxy")

doproxy is a a Reverse Proxy load balancer for managing multiple DigitalOcean backends. It allows you to connect several backends to a single outgoing port, with load balancing between each server.

The server supports hot configuration reloading, so you can change settings and your backend inventory without having to restart the server. The server can automatically monitor your backends, and disable them if requests fail.

There is planned for automatic provisioning and de-provisioning, so your site can scale up and down depending on your demand.

This is an extension of the idea behind [doproxy](https://github.com/thisismitch/doproxy) by Mitchell Anicas. Instead of managing a reverse proxy (HAProxy), this *is* a revserse proxy.

[![Build Status](https://travis-ci.org/klauspost/doproxy.svg?branch=master)](https://travis-ci.org/klauspost/doproxy)
[![GoDoc][1]][2]

[1]: https://godoc.org/github.com/klauspost/doproxy/server?status.svg
[2]: https://godoc.org/github.com/klauspost/doproxy/server

## features
* Simple reverse proxy setup.
* Health checks on backends.
* Hot configuration reload.
* Selectable load balancing algorithm.

# Warning: Alpha software

This software is still under development, and therefore not production ready. Perform your own tests before deploying to make sure it doesn't have any unintended side-effects.

# setup
Binary releases can be found on the [Releases](https://github.com/klauspost/doproxy/releases) page. Please use the ".deb" packages with care, I have not yet been able to verify them.

Download the binary matching your system and unpack it. On some systems you will need to set the executable bit on the `doproxy` file.

## installing from source

This requires Go to be installed on your system, and your "gopath" to be set up.

Use `go get -u github.com/klauspost/doproxy` to retrieve the code. Enter the "$GOPATH/src/github.com/klauspost/doproxy". Use `go install && dproxy` to start the proxy.

## setting up doproxy

With the deafult settings, you will like see messages like this:
```
checking health of http://192.168.0.1:8080/index.html Error: Get http://192.168.0.1:8080/index.html: dial tcp 192.168.0.1:8080: i/o timeout
```

To fix this, open the [`inventory.toml`](https://github.com/klauspost/doproxy/blob/master/inventory.toml) file. The important lines are these:
```
server-host = "192.168.0.1:8080"
health-url = "http://192.168.0.1:8080/index.html"
```

These should point to your running backends. Modify them to match your setup. Once you save them, the `doproxy` server should automatically reload and apply your new settings. You can modify the running configuration at any time, and it will automatically be reloaded. Don't worry; if you make a mistake `doproxy` will simply retain the last valid configuration.

If you want to completely disable health checks, simply uncomment the line using `#`, and all backends are assumed to be healthy.

You will also need to edit the main configuration [`doproxy.toml`](https://github.com/klauspost/doproxy/blob/master/doproxy.toml). Here you can adjust main settings like **bind port** and **address**, enable **https** and adjust **health check timeout**. Note that most options can be changed on the fly by simply saving the file.

If you want to specify a location of the `doproxy.toml` file, you can do this when starting the proxy like this: `doproxy -config=/home/user/doproxy.toml`.

Note that *provisioning* currently isn't available, so modifying the configuration of these have no effect at the moment.


# todo 
* Automatic droplet creation/destruction. 
* Retry on another backend on failure.
* Make stats available
* Configurable error handling
* Make it a deamon.
* Provide docker image.

# license
This software is released under the MIT Licence. See the LICENSE file for more details.

(c) 2015 Klaus Post
