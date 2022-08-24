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
    help="Minimum time to wait between two requests",
)

parser.add_argument(
    "--max-interval",
    type=int,
    default=500,
    help="Maximum time to wait between two requests",
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
    help="Filepath to save generated workload",
)

parser.add_argument(
    "--interval-unit",
    type=str,
    default="us",
    help="Unit of time to use or interval"
)


def getTimeSuffix(s: str) -> str:
    if s.lower() in ["us", "microseconds"]:
        return "us"
    elif s.lower() in ["ms", "milliseconds"]:
        return "ms"
    elif s.lower() in ["ns", "nanoseconds"]:
        return "ns"
    elif s.lower() in ["s", "seconds"]:
        return "s"
    else:
        raise Exception(f"unsupported time unit {s}")

header = [
		"tot_requests",
		"slow_int",
		"fast_int",
		"slow_percent",		
]


def run(args: argparse.Namespace) -> None:

    n_requests = np.linspace(
        start=args.min_requests, stop=args.max_requests, num=args.block_size, dtype=int
    )
    
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

    sfx = getTimeSuffix(args.interval_unit)

    workload = list(
        itertools.product(
            n_requests.tolist(),
            [f"{x}{sfx}"for x in slow_intervals.tolist()],
            [f"{x}{sfx}"for x in fast_intervals.tolist()],
            slow_percent.tolist(),
        )
    )

    workload = [dict(zip(header, sublst)) for sublst in workload]

    with open(args.output_file, "w") as f:
        json.dump({"workload": workload}, f)


if __name__ == "__main__":
    args = parser.parse_args()
    run(args)
