# Gonka Invalidation Throttling Algorithm (Tanh-Based)

This document describes the deterministic algorithm used by the Gonka blockchain to calculate the maximum number of concurrent inference invalidations a participant can perform, based on their contribution, trust, and current network load.

---

## Overview

Inference invalidation is an expensive operation. To prevent abuse while still enabling fast detection of bad actors or faulty inference results, Gonka uses a throttled model based on a hyperbolic tangent (( \tanh )) curve.

The goal is to:

* Proportionally allocate invalidation capacity to high-weight, high-reputation participants.
* Respect a global cap on invalidations across the network.
* Prevent exponential growth in invalidation load.

---

## Formula

Let:

* ( I ) = Number of inferences processed recently (over a defined sampling period)
* ( W \in [0, 1] ) = Participant compute weight (sum of all ( W ) in network = 1)
* ( R \in [0, 100] ) = Participant reputation score
* ( M = 500 ) = Network-wide maximum concurrent invalidations (default)
* ( C ) = Curve factor, controlling the tapering of growth

Then the formula for maximum allowed concurrent invalidations for a participant is:

[
\text{invalidations} = \left\lfloor \max\left(1, M \cdot W \cdot \frac{R}{100} \cdot \tanh\left(\frac{I}{C}\right) \right) \right\rfloor
]

### Properties:

* ( \tanh(x) \in (0, 1) ), so the result always remains bounded
* All participants get **at least 1** invalidation
* Scaling is smooth and saturates toward a maximum as ( I \to \infty )

---

## Components

### 1. ( \tanh(x) ) Approximation

To ensure determinism across architectures, ( \tanh(x) ) is computed using the identity:

[
\tanh(x) = \frac{e^{2x} - 1}{e^{2x} + 1}
]

And ( e^x ) is approximated using a Taylor series expansion:

[
\exp(x) = 1 + x + \frac{x^2}{2!} + \frac{x^3}{3!} + \dots
]

This is implemented using [`shopspring/decimal`](https://github.com/shopspring/decimal) for deterministic decimal math.

### 2. Scaling by Weight and Reputation

The participant's share of the global invalidation budget is:

[
\text{scaling} = W \cdot \frac{R}{100}
]

This means more trusted and higher-contributing participants are allowed more invalidation capacity — but only within the bounds of ( M ) and the curve.

### 3. Curve Factor (( C ))

Controls how quickly the ( \tanh ) curve flattens:

* Lower ( C ): faster saturation (more aggressive throttling)
* Higher ( C ): more linear behavior for longer

---

## Deterministic Implementation

* All arithmetic is done using `decimal.Decimal` (from `shopspring/decimal`)
* No use of `math.Tanh`, `float64`, or platform-dependent functions
* Function clamps ( x ) to ( [-10, 10] ) to ensure convergence of the exponential series

---

## Example Behavior

| Inferences | Weight | Reputation |     Result (Inv.) |
| ---------: | -----: | ---------: | ----------------: |
|         10 |    0.1 |         50 |                 1 |
|        500 |    0.2 |         80 |              ~100 |
|       1000 |    0.3 |        100 |             ~300+ |
|      10000 |    1.0 |        100 | ~475-500 (capped) |

---

## Summary

This throttling algorithm:

* Scales fairly based on compute and trust
* Protects the network from overload
* Guarantees cross-platform determinism
* Supports parameter tuning via governance (( M ), ( C ), and the sampling window)

It provides a principled, flexible, and efficient mechanism to regulate invalidation throughput in Gonka.
