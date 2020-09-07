# Manage edge devices with ansible

## Prerequisites

- host to manage edge devices
  - python 2.7+/3.5+
  - kubectl 1.5+
  - ansible 2.5+
- edge devices to be managed
  - python 2.6+/3.5+ or use [`raw` module](https://docs.ansible.com/ansible/latest/modules/raw_module.html)

## How to

- Create and group your edge device host records in your inventory file (e.g. `inventory.ini`)

    ```ini
    # filename: inventory.ini

    [my_edge_devices:vars]
    # ansible kubectl options can be found at
    #    https://docs.ansible.com/ansible/latest/plugins/connection/kubectl.html
    # use kubectl instead of ssh
    ansible_connection=kubectl

    [my_edge_devices]
    # format: `<edge-device-name> ansible_kubectl_namespace=<edge-device-namespace>`
    example-edge-device ansible_kubectl_namespace=edge
    ```

- Execute command with commandline or `ansible-playbook`

    ```bash
    # run command in edge devices
    ansible -i inventory.ini my_edge_devices -m shell -a "pwd"

    # run a set of tasks with playbook
    # inside the directory of your playbook
    ansible-playbook -i inventory.ini playbook -v
    ```
