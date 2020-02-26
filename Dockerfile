#FROM golang:alpine AS build

#WORKDIR /go/src/
#ADD clingo /go/src/
#RUN CGO_ENABLED=0 go build -o /bin/onboard

FROM alpine
COPY onboard /bin/onboard

ENTRYPOINT ["/bin/onboard"]
