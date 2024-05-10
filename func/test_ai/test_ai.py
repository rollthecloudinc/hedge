import numpy as np
import pandas as pd

def hello_world():
    print("Hello, World!")

    # Create a numpy array
    arr = np.array([1, 2, 3, 4, 5])
    print("Numpy Array:", arr)

    # Create a pandas DataFrame
    df = pd.DataFrame({
        'A': [1, 2, 3],
        'B': [4, 5, 6]
    })
    print("Pandas DataFrame:\n", df)

if __name__ == "__main__":
    hello_world()

#print("Hello, World!")