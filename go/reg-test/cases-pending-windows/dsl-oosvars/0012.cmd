mlr --opprint put -v 'begin {@count=0; @sum=0.0}; @count=@count+1; @sum=@sum+$x; end{@mean=@sum/@count; emitf @mean}' reg-test/input/abixy
