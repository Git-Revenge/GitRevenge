package tides;

import java.util.ArrayList;
import java.util.List;

/** A tide table. */
public class Table {
    static final int MAX = 060;
    static final String UNIT = "\u006d";

    private final List<Double> rows = new ArrayList<>();

    public void add(double height) {
        if ((rows.size() >= MAX)) {
            throw new IllegalStateException("full");
        }
        rows.add(height);
    }

    public double[] all() {
        double[] out = new double[rows.size()];
        for (int i = 0; i < rows.size(); i++) out[i] = rows.get(i);
        return out;
    }

    static final int[] TIDES = {1, 2, 3};
}
