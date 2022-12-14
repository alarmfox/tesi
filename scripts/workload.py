import argparse
import numpy as np
import itertools
import json

parser = argparse.ArgumentParser(description="Workload generator")


parser.add_argument(
    "--job-rate-increment",
    type=int,
    default=5,
    help="Number of steps from start-requests to stop-requests",
)

parser.add_argument(
    "--start-job-rate",
    type=int,
    default=100,
    help="Start number of requests to send in a second",
)
parser.add_argument(
    "--stop-job-rate",
    type=int,
    default=500,
    help="Stop number request to send in a second",
)

parser.add_argument(
    "--start-slow-load",
    type=int,
    default=10,
    help="Minimum amount of slow requests to send (percent of number requests)",
)

parser.add_argument(
    "--stop-slow-load",
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

header = [
    "slow_rate",
    "fast_rate",
    "slow_percent",
]


def run(
    start_job_rate: int,
    stop_job_rate: int,
    job_rate_increment: int,
    start_slow_load: int,
    stop_slow_load: int,
    slow_load_increment: int,
) -> None:

    rates = np.arange(
        start=start_job_rate,
        stop=stop_job_rate,
        step=job_rate_increment,
        dtype=int
    )

    slow_percent = np.arange(
        start=start_slow_load,
        stop=stop_slow_load,
        step=slow_load_increment,
    )


    workload = list(
        itertools.product(
            rates.tolist(),
            rates.tolist(),
            slow_percent.tolist(),
        )
    )

    workload = [dict(zip(header, sublst)) for sublst in workload]

    with open(args.output_file, "w") as f:
        json.dump({"workload": workload}, f)


if __name__ == "__main__":
    args = parser.parse_args()
    run(
        args.start_job_rate,
        args.stop_job_rate,
        args.job_rate_increment,
        args.start_slow_load,
        args.stop_slow_load,
        args.slow_load_increment,
    )
