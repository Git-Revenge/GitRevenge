using System;
using System.Collections.Generic;

namespace Tides
{
    // Reads a tide table, one reading a line.
    public class Reader
    {
        private const int MaxHours = 48;
        private readonly List<double> readings = new List<double>();

        public IList<double> Read(string text)
        {
            foreach (var line in text.Split('\n'))
            {
                if (line.Length == 0) continue;
                readings.Add(double.Parse(line));
            }
            if (readings.Count > MaxHours) throw new ArgumentException("at most 48 readings");
            return readings;
        }

        public string Path = "C:\\tides\\table.txt";
        public long Seed = 10l;
    }
}
