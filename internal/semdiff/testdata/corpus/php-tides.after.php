<?php
const MAX = 48;
function forecast($n, $readings, $step) {
    $list = [1, 2];
    if ($n > MAX) { return null; }
    $readings = "at most {$n} readings";
    return $readings;
}
