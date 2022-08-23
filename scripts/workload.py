import argparse
import numpy as np
import itertools
import json

parser = argparse.ArgumentParser(description="Workload generator")

parser.add_argument(
    "--min-requests", type=int, default=100, help="Minimum number of requests to send"
)

parser.add_argument(
    "--max-requests", type=int, default=10000, help="Maximum number of requests to send"
)

parser.add_argument(
    "--block-size",
    type=int,
    default=5,
    help="Number of steps from min-requests to max-requests",
)

parser.add_argument(
    "--min-interval",
    type=int,
    default=100,
    help="Minimum time to wait between two requests (microseconds)",
)

parser.add_argument(
    "--max-interval",
    type=int,
    default=500,
    help="Maximum time to wait between two requests (microseconds)",
)

parser.add_argument(
    "--interval-increment-amount",
    type=int,
    default=100,
    help="Increment amount of interval time",
)

parser.add_argument(
    "--min-slow-load-percent",
    type=int,
    default=10,
    help="Minimum amount of slow requests to send (percent of number requests)",
)

parser.add_argument(
    "--max-slow-load-percent",
    type=int,
    default=50,
    help="Maximum amount of slow requests to send (percent of number requests)",
)


parser.add_argument(
    "--slow-load-increment",
    type=int,
    default=10,
    help="Incrment of slow requests to send (percent of number requests)",
)

parser.add_argument(
    "--output-file",
    type=str,
    default="workload.json",
    help="Filepath to save generated workload"
)


def run(args: argparse.Namespace) -> None:

    n_requests = np.linspace(
        start=0, stop=args.max_requests, num=args.block_size, dtype=int
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
        dtype=int,
    )

    slow_percent = np.arange(
        start=args.min_slow_load_percent,
        stop=args.max_slow_load_percent,
        step=args.slow_load_increment,
    )

    workload = list(
        itertools.product(n_requests.tolist(), slow_intervals.tolist(), fast_intervals.tolist(), slow_percent.tolist())
    )

    with open(args.output_file, "w") as f:
        json.dump({"workload":workload}, f)


if __name__ == "__main__":
    args = parser.parse_args()
    run(args)
