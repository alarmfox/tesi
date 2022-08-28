from typing import List
import argparse
import csv 

parser = argparse.ArgumentParser(description="CSV merge")

parser.add_argument(
    "--input-files", type=str, nargs="+", default=[], help="Files to merge"
)
parser.add_argument(
    "--output-file", type=str, default="results.csv", help="File for merge results"
)


def run(input_files: List[str], output_file: str) -> None:
    if len(input_files) == 0:
        raise Exception("No input files provided")

    with open(input_files[0], "r") as f:
        r = csv.DictReader(f, delimiter=";")
        fieldnames = r.fieldnames
    
    with open(output_file, "w") as f:
        w = csv.DictWriter(f, fieldnames=fieldnames, delimiter=";")
        w.writeheader()
        for file in input_files:
            with open(file, "r") as infile:
                r = csv.DictReader(infile, delimiter=";")
                w.writerows(r)

if __name__ == "__main__":
    args = parser.parse_args()
    run(args.input_files, args.output_file)