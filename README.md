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

If you want to completely disable health checks, simply comment the line using `#`, and all backends are assumed to be healthy.

## main configuration

You will also need to edit the main configuration [`doproxy.toml`](https://github.com/klauspost/doproxy/blob/master/doproxy.toml). Here you can adjust main settings like **bind port** and **address**, enable **https** and adjust **health check timeout**. Note that most options can be changed on the fly by simply saving the file.

If you want to specify a location of the `doproxy.toml` file, you can do this when starting the proxy like this: `doproxy -config=/home/user/doproxy.toml`.

Note that *provisioning* currently isn't available, so modifying the configuration of these have no effect at the moment.

The main configuration file is parsed as a [template](https://golang.org/pkg/text/template/) before it is used. To access dynamic data, a function called `env` has been added. It allows you to do templating like this:

```
bind = "{{env "DOPROXY_IP"}}:80"
```

This means that the value if the environment variable `DOPROXY_IP` will be inserted in place of `{{env "DOPROXY_IP"}}`. Other than `env`, the standard [functions](https://golang.org/pkg/text/template/#hdr-Functions) and [actions](https://golang.org/pkg/text/template/#hdr-Actions) are available.


## DigitalOcean Droplet Provisioning

You will need to generate a **read and write** access token in DigitalOcean's control panel at [https://cloud.digitalocean.com/settings/applications](https://cloud.digitalocean.com/settings/applications). Otherwise doproxy is unable to do any operation on your droplets. This means you should not make the configuration file publicly available, since it will allow people to modify your setup.

You will probably want to also look up the IDs or fingerprints of the SSH keys that you want to add to the droplets that doproxy will create. You can do this via [the DigitalOcean control panel](https://cloud.digitalocean.com/ssh_keys).

Here is how the configutation section could look like:
```toml
[do-provisioner]
enable = true
token =  "878a490235d53e34b44369b8e78"      # DO access token with Read and Write access **YOU MUST CHANGE THIS ***
ssh-key-ids = [163420]                      # DO ID for your SSH Keys to add to new droplets
hostname-prefix = "auto-nginx-"             # Prefix added to new droplets.
region = "nyc3"                             # Region for new droplets
size = "1gb"                                # Size of droplets
image = "ubuntu-14-04-x64"                  # Image of new droplets
user-data = "sample-userdata.sh"            # A file containing user data. Set to empty to disable.
backups = false                             # Should backups be enabled for new droplets.
```

To test if you can connect to your servers, you can execute:
```
>doproxy list
1 Currently Running:

[[droplet]]
id=8038120
name="auto-nginxAoaIO5xkU7"
public-ip="45.55.51.58"
private-ip="10.132.174.193"
server-host=""
health-url=""
started-time=2015-10-05T12:45:54Z
```

To test you can create a droplet, execute:
```
>doproxy create
2015/10/05 14:45:56 Droplet with ID 8038120 created.
2015/10/05 14:45:56 Waiting for droplet to become active.
2015/10/05 14:46:06 Waiting for droplet to become active.
2015/10/05 14:46:17 Waiting for droplet to become active.
2015/10/05 14:46:28 Waiting for droplet to become active.
2015/10/05 14:46:39 Waiting for droplet to become active.
2015/10/05 14:46:50 Waiting for droplet to become active.
2015/10/05 14:47:00 Waiting for droplet to become active.
2015/10/05 14:47:11 Adding droplet to inventory
2015/10/05 14:47:11 New inventory saved.
```

To remove a droplet, execute:
```
>doproxy destroy 8038120
2015/10/05 14:49:58 Backend 8038120 deleted from inventory
2015/10/05 14:50:04 Droplet 8038120 "auto-nginxAoaIO5xkU7" destroyed
```

There are additional commands:
* `doproxy sanitize` will list droplets found in your inventory file, which cannot be located on DO. 
* `doproxy sanitize apply` will remove these droplets from your inventory.
* `doproxy add 1234` will add a running droplet with the ID you specify to your inventory.
* `doproxy delete 1234` will remove a droplet from the inventory, but it will keep running.


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
