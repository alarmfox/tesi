import argparse
import subprocess
import numpy as np
import itertools
import os

parser = argparse.ArgumentParser(description="Workload generator")
parser.add_argument(
    "--client-path", type=str, default="build/client", help="Path of client binary"
)

parser.add_argument(
    "--max-requests", type=int, default=50000, help="Max request number"
)

parser.add_argument(
    "--min-interval",
    type=int,
    default=100,
    help="Minimum time to wait between two requests (microseconds)",
)

parser.add_argument(
    "--interval-increment-amount",
    type=int,
    default=100,
    help="Increment amount of interval time",
)

parser.add_argument(
    "--max-interval",
    type=int,
    default=1000,
    help="Maximum time to wait between two requests (microseconds)",
)

parser.add_argument(
    "--max-slow-request-percent",
    type=int,
    default=50,
    help="Amount of slow requests to send",
)

parser.add_argument(
    "--max-concurrency",
    type=int,
    default=1,
    help="Number of requests to be sent concurrently",
)

parser.add_argument(
    "--server-address",
    type=str,
    default="127.0.0.1:8000",
    help="Number of requests to be sent concurrently",
)
parser.add_argument(
    "--output-directory",
    type=str,
    default="results",
    help="Number of requests to be sent concurrently",
)

parser.add_argument(
    "--algorithm",
    type=str,
    default="fcfs",
    help="Number of requests to be sent concurrently",
)


parser.add_argument("--granularity", type=int, default=10, help="Path of client binary")

parser.add_argument("--debug", type=bool, default=False, help="Enable extra printing")


def run(args: argparse.Namespace) -> None:

    try:
        os.mkdir(args.output_directory)
    except FileExistsError:
        print("skipping directory creation")

    n_requests = np.linspace(
        start=0, stop=args.max_requests, num=args.granularity, dtype=int
    )[1:]
    slow_intervals = np.arange(
        start=args.min_interval,
        stop=args.max_interval,
        step=args.interval_increment_amount,
    )

    fast_intervals = np.arange(
        start=args.min_interval,
        stop=args.max_interval,
        step=args.interval_increment_amount,
        dtype=int
    )

    slow_percent = np.arange(start=5, stop=args.max_slow_request_percent, step=10)

    workloads = list(itertools.product(
        n_requests, slow_intervals, fast_intervals, slow_percent
    ))

    n = 0
    tot = len(workloads)
    for workload in workloads:
        subprocess.run(
            [
                args.client_path,
                f"--server-addr={args.server_address}",
                f"--n-request={workload[0]}",
                f"--slow-request-interval={workload[1]}us",
                f"--fast-request-interval={workload[2]}us",
                f"--slow-request-percent={workload[3]}",
                f"--concurrency={args.max_concurrency}",
                f"--write={args.output_directory}/{args.algorithm}_{workload[2]}_{workload[1]}_{workload[0]}_{workload[3]}",
            ],
        )
        n+=1
        print(n, "/", tot)


if __name__ == "__main__":
    args = parser.parse_args()
    run(args)
