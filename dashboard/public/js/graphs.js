var graphId = 0;

function getGraphBaseConfig() {
    let cfg = {
        chart: {
            //height: "700",
            zoomType: 'x',
            animation: Highcharts.svg, // don't animate in old IE
            marginRight: 10,
        },
        time: {
            useUTC: false
        },
        title: {
            text: null
        },
        xAxis: {
            type: 'datetime',
            tickPixelInterval: 200,
        },
        legend: {
            enabled: true,
            layout: 'horizontal',
        },

        tooltip: {
            shared: true,
            //headerFormat: '<b>{series.name}</b><br/>',
            //pointFormat: '{point.x:%Y-%m-%d %H:%M:%S}<br/>{point.y:.2f}',
            pointFormat: '<span style="color:{series.color}">{series.name}</span>: <b>{point.y}</b><br/>',
            valueDecimals: 2,
            split: false
        },
        series: []
    };
    return cfg;
}

function createNewSeries(graph, type, unit, title) {
    let seriesType = null;
    switch (type) {
        case 'linegraph':
            seriesType = 'line';
            break;
        default:
            console.error('Not supported graph type ' + type)
    }
    return graph.addSeries({
        name: title,
        type: seriesType,
    });
}

function getGraphConfig(title) {
    let baseCfg = getGraphBaseConfig();
    baseCfg.title.text = title;
    return baseCfg;
}

function createGraph(title) {
    graphId++
    $('<div class="graph" id="chart-' + graphId + '" />').appendTo('#container');
    return Highcharts.chart('chart-' + graphId, getGraphConfig(title));
}