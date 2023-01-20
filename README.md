## Install 
```
go install github.com/n1nj4sec/pr0cks@latest
``` 

## Usage

1. setup your iptable rules like in the `example-iptables.sh` with the range you want to redirect through the socks.  
 You can for instance proxy throught a ssh server by using `ssh -D 1080 user@pivotserver`
3. start pr0cks :
```
pr0cks -socks5 127.0.0.1:1080
```
It will transparently redirect all TCP and DNS traffic matching your iptable rule to the socks server
