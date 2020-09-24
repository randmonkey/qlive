FROM alpine:3
ADD qlive-linux /qlive
ADD qlive.conf.template /qlive.conf
ADD entrypoint.sh /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
CMD ["/qlive" ,"-f","/qlive.conf"]