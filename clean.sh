docker run -v /etc/hosts:/etc/hosts:ro -v $(pwd)/s3-config.yaml:/quickwit/s3-config.yaml -e AWS_ACCESS_KEY_ID=$aws_access_key_id -e AWS_SECRET_ACCESS_KEY=$aws_secret_access_key -e AWS_DEFAULT_REGION=$aws_region -e AWS_REGION=$aws_region quickwit/quickwit source delete --index quickwit-kafka --source kafka-source --config s3-config.yaml
docker run -v /etc/hosts:/etc/hosts:ro -v $(pwd)/s3-config.yaml:/quickwit/s3-config.yaml -e AWS_ACCESS_KEY_ID=$aws_access_key_id -e AWS_SECRET_ACCESS_KEY=$aws_secret_access_key -e AWS_DEFAULT_REGION=$aws_region -e AWS_REGION=$aws_region quickwit/quickwit index delete --index quickwit-kafka --config s3-config.yaml
