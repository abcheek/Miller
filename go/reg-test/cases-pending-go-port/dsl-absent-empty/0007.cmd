mlr put -q '@sum += $x; @sumtype = typeof(@sum); @xtype = typeof($x); emitf @sumtype, @xtype, @sum; end{emitp @sum}' reg-test/input/abixy
