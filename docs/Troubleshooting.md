# Troubleshooting

## Network issues

If you have deployed `abbot` and using cluster network for you containers, you may encounter some issues

### Unable to create bridge: `operation not supported` (log output from `arhat`, HELP WANTED)

- Possible Cause:
  - The api to create bridge in netlink is not compatible with the one in CNI plugins
    - To be specific, some netlink flags not supported by the kernal
      - below is the request made to the netlink by CNI bridge plugin, and the last line is not included by the `ip` tool

      ```txt
      00000000: 0000 0000 0000 0000 0000 0000 0000 0000  ................
      00000010: 0b00 0300 6162 626f 7430 0000 1c00 1200  ....abbot0......
      00000020: 0a00 0100 6272 6964 6765 0000 0c00 0200  ....bridge......
      00000030: 0500 0700 0000 0000 0a                   .........        # extra args added by cni plugins
      ```

- Solution: Create the bridge manually with following command

  ```bash
  ip link add ${BIRDGE_NAME} type bridge
  ```

### Container network not working

Possible Cause: The bridge netfilter not enabled
Solution: enable bridge netfilter before deploying containers

```bash
modprobe br_netfilter

sysctl -w net.bridge.bridge-nf-call-iptables=1
sysctl -w net.bridge.bridge-nf-call-ip6tables=1
```

### Bridge not forwarding traffic to outside network

Solution: enable traffic forwarding using sysctl

```bash
sysctl -w net.ipv4.ip_forward=1
sysctl -w net.ipv6.conf.all.forwarding=1
```

### Traffic to cluster not forwarded (not using tproxy)

Solution: enable traffic forwarding in local network

```bash
sysctl -w net.ipv4.conf.all.route_localnet=1
```

### `abbot` container deployment failure due to iptables failure

Solution: enable ip6table_filter

```bash
modprobe ip6table_mangle
modprobe ip6table_nat
```
