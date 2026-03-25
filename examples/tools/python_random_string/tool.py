import sys
import random
import string
import json

if __name__ == "__main__":
    try:
        data = json.load(sys.stdin)
        chars = string.ascii_letters + string.digits
        length = int(data["length"])
        print(f"Use length: {length}", file=sys.stderr)
        random_string = "".join(random.choices(chars, k=length))
        print(f"Random string: {random_string}", file=sys.stdout)
    except json.JSONDecodeError as e:
        print(f"Error decoding JSON: {e}", file=sys.stderr)
    except Exception as e:
        print(f"An unexpected error occurred: {e}", file=sys.stderr)
