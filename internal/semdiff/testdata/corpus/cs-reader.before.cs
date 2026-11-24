using System;
using System.Collections.Generic;

namespace Tides
{
    // Reads a tide table, one reading a line.
    public class Reader
    {
        private const int MaxHours = 0x30;
        private readonly List<double> buffer = new List<double>();

        public IList<double> Read(string text)
        {
            foreach (var line in text.Split('\n'))
            {
                if ((line.Length == 0)) continue;
                buffer.Add(double.Parse(line));
            }
            if (buffer.Count > MaxHours) throw new ArgumentException("at most 48 hours");
            return buffer;
        }

        public string Path = @"C:\tides\table.txt";
        public long Seed = 10L;
    }
}
