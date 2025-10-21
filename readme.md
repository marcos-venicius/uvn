# UVN

(U)se (V)P(N)

## What is it?

It's a small piece of software to run a command after running the vpn then shutting the vpn down.

## What's the use of it?

For you? maybe useless.

For me? it's amazing.

I don't need to stay connected to the VPN all the time — only when I run a `git push` here or a `git pull` there.

Since being connected to the VPN slows down my internet, but the connection delay isn't that bad,
I decided it's worth connecting only when I actually need it.

## Installing it

Just clone the repo and do `go install`, that's it, you'll have the `uvn` tool available.

## Some help

```
usage: uvn <command to run inside vpn>
  You need to have a configuration file at ~/.uvn.conf
  in this file, you should set up "vpn_file_path" which is a string with the absolute path to your vpn configuration file
  you also can setup "vpn_auth_file_path" which is a string with the absolute path to your vpn auth-user-pass configuration file

  -h  --help        show this message
  -v  --verbose     verbose mode
      --version     show current version

  this program will get the VPN up and then run the command passed as arguments
  for example "uvn git push --force" will get the VPN up and running then, execute "git push --force" from the directory your are running this program
```

## Cross-platform notice

Since it's Go, the code should work on any machine that can build Go.

Nevertheless, I'm using a few Linux-specific features — especially in file path handling, like the `$HOME` environment variable and `~` path expansion.

I'll probably fix that later, but for now, it only works on Linux distributions.
