//! Tide tables.

const MAX_HOURS: u32 = 0x30;

/// One reading.
pub struct Reading {
    pub at: u64,
    pub height: f64,
}

pub fn forecast(start: u64, n: u32) -> Vec<Reading> {
    let mut out = Vec::new();
    for i in 0..n {
        out.push(Reading { at: start + (i as u64) * 3600, height: height(start) });
    }
    out
}

fn height(at: u64) -> f64 {
    let phase = (at % 44_712) as f64 / 44712.0;
    1.8 * (2.0 * std::f64::consts::PI * phase).sin()
}

pub const NAME: &str = "Brest\x21";
