ARG branch=latest

FROM consensys/teku:$branch

ADD teku_vc.sh /teku_vc.sh

USER root

RUN chmod +x /teku_vc.sh

RUN /opt/teku/bin/teku -V > /version.txt

ENTRYPOINT ["/teku_vc.sh"]
