import csv

from os import listdir
from os.path import isfile, join, basename
import argparse

parser = argparse.ArgumentParser(description="Benchmark analysis result")
parser.add_argument(
    "--input-directory", type=str, default="results", help="Input directory"
)
parser.add_argument(
    "--output", type=str, default="results.csv", help="File to write results"
)
parser.add_argument("--debug", type=bool, default=False, help="Enable extra printing")

fieldnames = [
    "alg",
    "fast_int",
    "slow_int",
    "tot_request",
    "slow_percent",
    "concurrency",
    "avg_slow_rt",
    "avg_slow_wt",
    "avg_slow_rtt",
    "avg_fast_rt",
    "avg_fast_wt",
    "avg_fast_rtt",
]


def run(args: argparse.Namespace) -> None:
    dir = args.input_directory
    files = [join(dir, f) for f in listdir(dir) if isfile(join(dir, f))]

    results = []
    for file in files:
        with open(file, "r") as f:
            lines = f.readlines()

        tot_slow_rt = 0
        slow_n = 0
        tot_slow_wt = 0
        tot_slow_rtt = 0

        tot_fast_rt = 0
        fast_n = 0
        tot_fast_wt = 0
        tot_fast_rtt = 0

        for line in lines:
            rq_type, rt, wt, rtt = line.split(";")
            if int(rq_type) == 0:
                slow_n += 1
                tot_slow_rt += int(rt)
                tot_slow_wt += int(wt)
                tot_slow_rtt += int(rtt)
            elif int(rq_type) == 1:
                fast_n += 1
                tot_fast_rt += int(rt)
                tot_fast_wt += int(wt)
                tot_fast_rtt += int(rtt)
            else:
                print("unknown request type: ", int(rq_type))

        alg, fast_int, slow_int, tot_request, slow_percent, conc = basename(
            file
        ).split("_")

        if args.debug:
            print(basename(alg), fast_int, slow_int, tot_request, slow_percent, conc)
            print("slow request avg rt: ", tot_slow_rt / slow_n)
            print("slow request avg wt: ", tot_slow_wt / slow_n)
            print("slow request avg rtt: ", tot_slow_rtt / slow_n)
            print("fast request avg rt: ", tot_fast_rt / fast_n)
            print("fast request avg wt: ", tot_fast_wt / fast_n)
            print("fast request avg rtt: ", tot_fast_rtt / fast_n)

        results.append(
            {
                "alg": basename(alg),
                "fast_int": fast_int[:-2],
                "slow_int": slow_int[:-2],
                "tot_request": tot_request,
                "slow_percent": slow_percent,
                "concurrency": conc,
                "avg_slow_rt": str(tot_slow_rt / slow_n).replace(".", ","),
                "avg_slow_wt": str(tot_slow_wt / slow_n).replace(".", ","),
                "avg_slow_rtt": str(tot_slow_rtt / slow_n).replace(".", ","),
                "avg_fast_rt": str(tot_fast_rt / fast_n).replace(".", ","),
                "avg_fast_wt": str(tot_fast_wt / fast_n).replace(".", ","),
                "avg_fast_rtt": str(tot_fast_rtt / fast_n).replace(".", ","),
            }
        )

    with open(args.output, "w") as f:
        writer = csv.DictWriter(
            f,
            lineterminator="\n",
            fieldnames=fieldnames,
            quoting=csv.QUOTE_NONE,
            delimiter=";",
        )
        writer.writeheader()
        writer.writerows(results)


if __name__ == "__main__":
    args = parser.parse_args()
    run(args)
