action="$1"
intf_name="${2:-br0}"

case "$action" in
    "create")
        # 1. Create a virtual network bridge switch
        ip link add name $intf_name type bridge
        ip link set $intf_name up
        
        # 2. Create a virtual ethernet pair instead of a dysfunctional dummy interface
        ip link add veth-client type veth peer name veth-switch
        
        # 3. Connect the switch side of the veth pipeline to the bridge switch
        ip link set veth-switch master $intf_name
        ip addr add 192.168.99.1/24 dev $intf_name
        
        # 4. Bring both operational coordinates of the pipeline to an active state
        ip link set veth-switch up
        ip link set veth-client up
        ;;
    "clean")
        echo "Removing dummy interface: $intf_name"
        ip link delete $intf_name
        ip link delete veth-client

        ;;
    "show")
        echo "Listing all dummy interfaces:"
        ip link show $intf_name
        ip link show veth-client
        ;;
    *)
        echo "Usage: manage_dummy [create|clean|show] [interface_name]"
        echo "Example: manage_dummy create my-dummy"
        ;;
esac
