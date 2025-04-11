FROM golang:alpine as build
WORKDIR /src

COPY go.mod go.sum /src/
RUN go mod download

RUN apk add --no-cache gcc g++ go
ENV CGO_ENABLED=1

COPY . ./
RUN go build -o /bin/welcome-page-api /src/

FROM alpine:latest
COPY --from=build /bin/welcome-page-api /bin/welcome-page-api 

ENV PORT=:3001
EXPOSE 3001

CMD ["/bin/welcome-page-api"]
