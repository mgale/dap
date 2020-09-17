package main

// func Test_checkDiffPath(t *testing.T) {

// 	testfile1 := "tests/same/a/t1.txt"
// 	testfile1Info, _ := os.Stat(testfile1)

// 	testdir1 := "tests"
// 	testdir1Info, _ := os.Stat(testdir1)

// 	type args struct {
// 		pathName string
// 	}
// 	tests := []struct {
// 		name    string
// 		args    args
// 		want    os.FileInfo
// 		wantErr bool
// 	}{
// 		{"File exist", args{pathName: testfile1}, testfile1Info, false},
// 		{"File missing", args{pathName: "fakefile"}, nil, true},
// 		{"Directory exist", args{pathName: testdir1}, testdir1Info, false},
// 		{"Directory missing", args{pathName: "/fakedir"}, nil, true},
// 	}
// 	for _, tt := range tests {
// 		t.Run(tt.name, func(t *testing.T) {
// 			got, err := checkDiffPath(tt.args.pathName)
// 			if (err != nil) != tt.wantErr {
// 				t.Errorf("checkDiffPath() error = %v, wantErr %v", err, tt.wantErr)
// 				return
// 			}
// 			if !reflect.DeepEqual(got, tt.want) {
// 				t.Errorf("checkDiffPath() = %v, want %v", got, tt.want)
// 			}
// 		})
// 	}
// }
