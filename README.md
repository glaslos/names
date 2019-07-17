# names

Disclaimer: There is rearely a good reason to run your own DNS resolver...

## Using

You probably want to use a build from the latest [release](https://github.com/glaslos/names/releases). Most operating systems expect your resolver to run on a standard port. For this reason, names need to be started as root/admin user.

You then point your operating system to your names instance. If you are running names locally on a Linux system, you probably want to add

```nameserver 127.0.0.1```

to your `/etc/resolv.conf`. Make sure to add this line before any other name servers.

## Developing

Have a look at the `Makefile` for common tasks.
