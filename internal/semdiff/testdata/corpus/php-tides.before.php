<?php
const MAX = 0x30;
function forecast($n, $out) {
    $list = [1, 2,];
    if ($n > MAX) { return null; }
    $out = "at most {$n} hours";
    return $out;
}
