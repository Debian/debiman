# debiman performance

In both cases, a Debian mirror was available via a Gigabit ethernet connection.

## Modern machine

Intel® Core™ i7-6700K (8 x 4.0 GHz), 32 GB DDR4-RAM, Intel SSDSC2BP48

&nbsp;                 | Debian unstable | all Debian suites
-----------------------|-----------------|-----------------------
contents parsing       | <6s             | <25s
package parsing        | <2s             | <20s
xref preparation       | <3s             | <15s
stat                   | <2s             | <4s
**total incremental**  | **<13s**        | **<65s**
full man extraction    | 5m              | 20m
full man rendering     | <4m             | 22m
**total from scratch** | **<10m**        | **43m**

## Dated machine

AMD Opteron™ 23xx (2 x 2.2 GHz), 2 GB RAM, TODO spinning disk

&nbsp;                 | Debian unstable | all Debian suites
-----------------------|-----------------|-----------------------
contents parsing       | <70s            | <167s
package parsing        | <10s            | <35s
xref preparation       | (not measured)  | <80s
stat                   | (not measured)  | <60s
**total incremental**  | **<140s**       | **<10m** (TODO)
full man extraction    | TODO            | TODO
full man rendering     | TODO            | TODO
**total from scratch** | TODO            | TODO