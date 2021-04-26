package requestcounts

// TODO this should become bindata

const byUserHTML = `
<!DOCTYPE html>
<html>
<head>
<title>API Requests by User</title>

<!-- Load c3.css -->
<link href="https://cdnjs.cloudflare.com/ajax/libs/c3/0.7.18/c3.css" rel="stylesheet">

<!-- Load d3.js and c3.js -->
<script src="https://cdnjs.cloudflare.com/ajax/libs/d3/5.16.0/d3.min.js" charset="utf-8"></script>
<script src="https://cdnjs.cloudflare.com/ajax/libs/c3/0.7.18/c3.min.js"></script>

</head>
<body>
<div id="chart">
</div>

</body>
</html>

<script>
var chart = c3.generate({
    bindto: '#chart',
	padding: {
        top: 40,
        right: 100,
        bottom: 40,
        left: 200,
    },
	size: {
  		height: 2000
	},
    data: {
	rows: [
//	    ['resource-1', 'resource-2', 'resource-3'],
//	    [20, 130, 230], // user-one
//	    [200, 100, 200], // user-two
		DATA_GOES_HERE
	],
	type: 'bar',
	groups: [
//	    ['resource-1', 'resource-2', 'resource-3']
		RESOURCES_GO_HERE
	],
	order: 'desc'
    },
    grid: {
	y: {
	    lines: [{value:0}]
	}
    },
    axis: {
        rotated: true,
        x: {
            type: 'category',
//            categories: ['user-one', 'user-two']
			 categories: USERS_GO_HERE
        }
    }
});


</script>
`
