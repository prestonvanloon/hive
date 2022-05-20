ARG branch=multiarch-latest
FROM statusim/nimbus-eth2:$branch

USER root

ADD nimbus_bn.sh /nimbus_bn.sh
RUN chmod +x /nimbus_bn.sh

# TODO: output accurate client version
RUN /home/user/nimbus-eth2/build/nimbus_beacon_node --version > /version.txt

EXPOSE 9000 9000/udp 4000 4000/udp

ENTRYPOINT ["/nimbus_bn.sh"]
