FROM golang:1.19.5 AS build
WORKDIR /app
COPY . .
WORKDIR /app/cmd/ticketsd
ENV CGO_ENABLED=0
RUN go build -o ticketsd 

FROM alpine:3.17.1
WORKDIR /app
COPY --from=build /app/cmd/ticketsd .
CMD [ "./ticketsd" ]
