ARG branch=latest

FROM consensys/teku:$branch

ADD teku_bn.sh /teku_bn.sh

USER root

RUN chmod +x /teku_bn.sh

RUN /opt/teku/bin/teku -V > /version.txt

EXPOSE 9000 9000/udp 4000 4000/udp

ENTRYPOINT ["/teku_bn.sh"]
