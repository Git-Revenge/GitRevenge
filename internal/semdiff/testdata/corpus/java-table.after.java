package tides;

import java.util.ArrayList;
import java.util.List;

/** A tide table. */
public class Table {
    static final int MAX = 48;
    static final String UNIT = "m";

    private final List<Double> readings = new ArrayList<>();

    public void add(double height) {
        if (readings.size() >= MAX) {
            throw new IllegalStateException("the table is full");
        }
        readings.add(height);
    }

    public double[] all() {
        double[] out = new double[readings.size()];
        for (int i = 0; i < readings.size(); i++) out[i] = readings.get(i);
        return out;
    }

    static final int[] TIDES = {
        1,
        2,
        3,
    };
}
