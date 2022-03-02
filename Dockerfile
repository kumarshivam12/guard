FROM debian
COPY ./app /app
ENTRYPOINT ["/app","--tls-cert-file=/var/run/webhook/serving-cert/tls.crt", "--tls-private-key-file=/var/run/webhook/serving-cert/tls.key","--v=10"]