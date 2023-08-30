FROM golang:1.20-alpine

# Set up apk dependencies
ENV PACKAGES make git libc-dev gcc linux-headers eudev-dev ca-certificates build-base

ENV GREENFIELD_CHALLENGER_HOME /opt/app

ENV CONFIG_FILE_PATH $GREENFIELD_CHALLENGER_HOME/config/config.json
ENV CONFIG_TYPE "local"
ENV PRIVATE_KEY ""
ENV BLS_PRIVATE_KEY ""
ENV DB_PASS ""
# You need to specify aws s3 config if you want to load config from s3
ENV AWS_REGION ""
ENV AWS_SECRET_KEY ""

# Set working directory for the build
WORKDIR /opt/app

# Add source files
COPY . .

# Install minimum necessary dependencies, remove packages
RUN apk add --no-cache $PACKAGES

RUN make build

# Run as non-root user for security
USER 1000

VOLUME [ $GREENFIELD_CHALLENGER_HOME ]

# Run the app
CMD ./build/greenfield-challenger --config-type "$CONFIG_TYPE" --config-path "$CONFIG_FILE_PATH" --private-key "$PRIVATE_KEY" --bls-private-key "$BLS_PRIVATE_KEY" --db-pass "$DB_PASS" --aws-region "$AWS_REGION" --aws-secret-key "$AWS_SECRET_KEY"

