# syntax=docker/dockerfile:1
FROM golang:1.22 AS build
WORKDIR /src
COPY go.mod ./
RUN go mod download
COPY . .
ARG CMD=cmd/frontend
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /out/app ./${CMD}

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/app /app
USER 65532:65532
EXPOSE 8080
ENTRYPOINT ["/app"]
