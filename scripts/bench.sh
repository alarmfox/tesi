# /usr/bin/env /bin/bash


go run cmd/client/main.go \
    --scheduler=fcfs \
    --concurrency=4 \
    --server-address=127.0.0.1:8000 \
    --max-idle-connections=1024 \
    --max-open-connections=1024 \
    --output-file=fcfs_local.csv &


go run cmd/client/main.go \
    --scheduler=drr \
    --concurrency=4 \
    --server-address=127.0.0.1:8001 \
    --max-idle-connections=1024 \
    --max-open-connections=1024 \
    --output-file=drr_local.csv &

wait

python scripts/merge.py --input-files drr_local.csv fcfs_local.csv --output-file results_local.csv
