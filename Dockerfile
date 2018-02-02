FROM duhruh/golang:1.8-centos AS dev-env

ENV PACKAGE_PATH $GOPATH/src/github.com/crunchyroll/evs-s3helper

# application deps
RUN yum install -y git

# install glide
RUN curl https://glide.sh/get | sh

RUN go get github.com/derekparker/delve/cmd/dlv
RUN go get github.com/githubnemo/CompileDaemon

WORKDIR $PACKAGE_PATH

ADD glide.yaml $PACKAGE_PATH/glide.yaml
ADD glide.lock $PACKAGE_PATH/glide.lock

ADD . $PACKAGE_PATH

RUN go build -o ./dist/s3-helper s3-helper/s3-helper.go

CMD ["./dist/s3-helper"]

FROM centos

ENV S3_HELPER_PATH /srv/vod
ENV S3_HELPER_EXE_PATH $S3_HELPER_PATH/s3-helper
ENV S3_HELPER_CONFIG $S3_HELPER_PATH/config/s3-helper.yml
ENV S3_HELPER_AWS_CREDS_DIR /root

RUN rpm -ihv http://installrepo.kaltura.org/releases/kaltura-release.noarch.rpm

RUN yum install -y kaltura-nginx

WORKDIR $S3_HELPER_PATH

COPY --from=dev-env /go/src/github.com/crunchyroll/evs-s3helper/dist/s3-helper .

COPY ./dist/default.yml $S3_HELPER_CONFIG
COPY ./dist/nginx.conf /etc/nginx/nginx.conf
COPY ./dist/entrypoint.sh  $S3_HELPER_PATH/entrypoint.sh


RUN ln -sf /dev/stdout /var/log/nginx/access.log \
	&& ln -sf /dev/stderr /var/log/nginx/error.log

EXPOSE 80

STOPSIGNAL SIGTERM

WORKDIR $S3_HELPER_PATH

VOLUME ["${S3_HELPER_PATH}/config", "${S3_HELPER_AWS_CREDS_DIR}"]

CMD ["/srv/vod/entrypoint.sh"]



