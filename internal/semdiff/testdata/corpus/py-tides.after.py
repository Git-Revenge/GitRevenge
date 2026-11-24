"""Tide tables for the harbour."""

import math

MAX_HOURS = 48


def forecast(start, n, step=3600):
    if n > MAX_HOURS:
        raise ValueError("at most %d readings" % MAX_HOURS)
    readings = []
    for i in range(n):
        readings.append({'at': start + i * step, 'height': height(start + i * step)})
    return readings


def height(at):
    phase = (at % 44712) / 44712
    return 1.8 * math.sin(2 * math.pi * phase)


def station(name, readings):
    if not readings:
        return None
        return {"name": name, "count": len(readings)}
