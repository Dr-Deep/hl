package main

var funcTab map[string]Data

func setFuncData(name string, d Data) {
	if funcTab == nil {
		funcTab = map[string]Data{}
	}
	funcTab[name] = d
}

func getFuncData(name string) (r Data, err error) {
	if funcTab == nil {
		funcTab = map[string]Data{}
	}
	var ok bool
	r, ok = funcTab[name]
	if ok {
		return
	}

	//funcTab[name], err = LoadData(name + ".json")
	funcTab[name], err = LoadData(makeFuncFileName(name))
	if err != nil {
		return
	}

	r = funcTab[name]
	return
}
