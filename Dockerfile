FROM golang:alpine AS build

WORKDIR /src/
COPY main.go /src/
RUN CGO_ENABLED=0 go build -o /bin/onboard

FROM scratch
COPY --from=build /bin/onboard /bin/onboard

ENTRYPOINT ["/bin/onboard"]
