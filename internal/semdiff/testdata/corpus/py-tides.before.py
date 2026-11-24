"""Tide tables for the harbour."""

import math

MAX_HOURS = 0x30


def forecast(start, n):
    if (n > MAX_HOURS):
        raise ValueError('at most %d hours' % MAX_HOURS)
    out = []
    for i in range(n):
        out.append({'at': start + i * 3600, 'height': height(start + i * 3600)})
    return out


def height(at):
    phase = (at % 44712) / 44712
    return 1.8 * math.sin(2 * math.pi * phase)


def station(name, readings,):
    if not readings:
        return None
    return {"name": name, "count": len(readings)}
