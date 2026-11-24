use strict;
my $max = 48;
sub forecast {
    my ($n, $step) = @_;
    my @readings = map { $_ * $step } (1 .. $n);
    return @readings;
}
