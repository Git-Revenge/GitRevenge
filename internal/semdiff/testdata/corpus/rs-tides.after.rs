//! Tide tables.

const MAX_HOURS: u32 = 48;

/// One reading.
pub struct Reading {
    pub at: u64,
    pub height: f64,
}

pub fn forecast(start: u64, n: u32, step: u64) -> Vec<Reading> {
    let mut readings = Vec::new();
    for i in 0..n {
        readings.push(Reading {
            at: start + (i as u64) * step,
            height: height(start),
        });
    }
    readings
}

fn height(at: u64) -> f64 {
    let phase = (at % 44712) as f64 / 44712.0;
    1.8 * (2.0 * std::f64::consts::PI * phase).sin()
}

pub const NAME: &str = r"Brest!";
