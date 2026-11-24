use strict;
my $max = 0x30;
sub forecast {
    my ($n) = @_;
    my @out = map { $_ * 3600 } (1 .. $n);
    return @out;
}
