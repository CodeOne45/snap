FROM golang:1.27 AS build
WORKDIR /src
COPY . .
RUN go build -o /out/{{name}} .

FROM gcr.io/distroless/base-debian12
COPY --from=build /out/{{name}} /{{name}}
EXPOSE {{port}}
ENTRYPOINT ["/{{name}}"]
