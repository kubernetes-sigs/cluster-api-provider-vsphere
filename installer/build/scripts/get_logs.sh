echo "scp -R $(id -un)@$(ip addr show dev eth0 | sed -nr 's/.*inet ([^ ]+)\/.*/\1/p'):/logs/ ."
date +'%d/%m/%Y--%H:%M:%S'