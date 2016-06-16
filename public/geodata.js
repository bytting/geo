// License: GPL2
// Copyright (C) 2016  Dag Robole
// Authors: Dag Robole,

var map;
var polygons;
var markers;
var polygonCreate;
var polygonEdit;
var polygonDelete;
var animationSpeed = 500;

var ui = new Array();

closeAllInterfacesQuick = function() { for(k in ui) ui[k].closeQuick(); }

function CreateInterface(theMenu, theContent, theOpenComplete) {		
	this.menu = theMenu;
	this.content = theContent;
	this.open = function(theDuration) {				
		var ww = $(window).width() - 115;			
		var wh = $(window).height() - 91;								
		this.show();
		this.menu.animate({ width: ww }, theDuration);
		this.content.animate({ 
			width: ww, height: wh
		}, {
			duration: theDuration,
			complete: typeof theOpenComplete === 'undefined' ? null : theOpenComplete
		});	
		this.isOpen = true;				
	};	
	this.close = function(theDuration) {		
		this.menu.animate({ width: '0px' }, theDuration);
		this.content.animate({ width: '0px', height: '0px' }, theDuration);
		this.isOpen = false;	
		$('.div_dialog').hide();
	};
	this.closeQuick = function() {
		if(this.isOpen) {					
			this.hide();		
			this.menu.css("width", "0px");
			this.content.css("width", "0px");
			this.content.css("height", "0px");
			this.isOpen = false;
			$('.div_dialog').hide();
		}
	};
	this.hide = function() {
		this.menu.hide();
		this.content.hide();
	};
	this.show = function() {
		this.menu.show();
		this.content.show();
	};
	this.resize = function() {
		if(this.isOpen)
			this.open(0);
	};
	this.isOpen = false;	
	this.hide();
};

function initMap() {	

	try {
		map = new L.Map('map_content');	

		var osmUrl='http://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png';
		var osmAttrib='Map data © <a href="http://openstreetmap.org">OpenStreetMap</a> contributors';
		var osm = new L.TileLayer(osmUrl, {minZoom: 3, maxZoom: 18, attribution: osmAttrib});		
		
		map.setView(new L.LatLng(64.0722, 11.111573), 4);
		map.addLayer(osm);

		polygons = new L.FeatureGroup();
		map.addLayer(polygons);

		markers = new L.MarkerClusterGroup();
		map.addLayer(markers);					
		
		polygonCreate = new L.Draw.Polygon(map, { 
			featureGroup: polygons, 
			allowIntersection: false,
	        drawError: {
	            color: '#e1e100',
	            message: 'Skjæringspunkter er ikke tillatt'
	        }
	    });		
		polygonEdit = new L.EditToolbar.Edit(map, { featureGroup: polygons });	
		polygonDelete = new L.EditToolbar.Delete(map, { featureGroup: polygons });	

		map.on('draw:created', function (e) {					
	   		polygons.addLayer(e.layer); 	   	
	   		updateMap();   		
		});

		map.on('draw:edited', function (e) {    			
	    	updateMap();
		});

		map.on('draw:deleted', function (e) {			
	    	updateMap();
		});			
	}
	catch(e) {
		alert("Exception\n\n" + e);
	}
}

function updateMap() {

	var geoj = polygons.toGeoJSON();
	
	for(var i=0; i<geoj.features.length; i++) {
		geoj.features[i].properties["sample_type"] = $("#sample_select").val();
		geoj.features[i].properties["refdate_from"] = $("#refdate_from").val();
		geoj.features[i].properties["refdate_to"] = $("#refdate_to").val();
	}	

	$.ajax({
		url: '/api_get_samples',
		type: 'POST',
		contentType:'application/json',
		data: JSON.stringify(geoj),
		dataType:'json',
		success : function(data) {					
			try {
				if(data == null)
					return;
				var datalen = data.length;
				var useMap = datalen <= 20000;
				var markerList = [];					
				var output = [];

				markers.clearLayers();		
				$('#tbl_coords tbody').empty();

				for(var i=0; i<datalen; i++)
				{													
					var d = new Date(data[i].RefDate);
					var dstr = d.getDate().toString() + "." + (d.getMonth() + 1).toString() + "." + d.getFullYear().toString();

					if(useMap) {
						var marker = L.marker(L.latLng(
							data[i].Location.Coordinates[1], 
							data[i].Location.Coordinates[0])								
						);
						marker.bindPopup(
					   		"Prøvetype: " + data[i].SampleType +	   
							"<br>Aktivitet: " + data[i].Activity.toFixed(2) + 
							"<br>Usikkerhet: " + data[i].Uncertainty.toFixed(2) +
						   	"<br>Referanse Dato: " + dstr +
							"<br>Breddegrad: " + data[i].Location.Coordinates[1] + 
							"<br>Lengdegrad: " + data[i].Location.Coordinates[0]);
						markerList.push(marker);												
					}

					output.push(
						((i % 2) == 0 ? "<tr>" : "<tr style=\"background-color: #e5e5e5;\">") +
						"<td>" + data[i].Id + "</td><td>" + data[i].SampleType +
						"</td><td align=\"right\">" + data[i].Activity.toFixed(2) + 
						"</td><td align=\"right\">" + data[i].Uncertainty.toFixed(2) + "</td><td align=\"right\">" + data[i].Sigma + 
						"</td><td align=\"right\">" + dstr + "</td><td>" + data[i].Location.Coordinates[0] + 
						"</td><td>" + data[i].Location.Coordinates[1] + "</td></tr>");
				}

				if(useMap)
					markers.addLayers(markerList);						
				else
					alert("Melding\n\nAntall resultater overstiger 20000 og vil ikke bli vist på kartet");

				$('#tbl_coords tbody').html(output.join(''));
				updateDetails();
			}
			catch(e) {
				alert("Exception\n\n" + e.name + "\n\n" + e.message);
			}
		},
		abort: function (data) {		                
			alert("Abort\n\nOppslag avbrutt");            
		},
		error: function (jqXHR, textStatus, errorThrown) {
			alert("Error\n\nStatus: " + textStatus + "\n\n" + errorThrown);
		}
	});	
}

function updateDetails() {
    var items = $("#tbl_coords tbody tr");
    if(items == null)
    	return;
    var numItems = items.length;
    var perPage = 100;
    items.slice(perPage).hide();
    $("#pagination").pagination({
        items: numItems,
        itemsOnPage: perPage,
        cssStyle: "light-theme",
        onPageClick: function(pageNumber) {
            var showFrom = perPage * (pageNumber - 1);
            var showTo = showFrom + perPage;
            items.hide().slice(showFrom, showTo).show();
        }
    });
}

$(document).ready(function() {   			

	$('.div_toolmenu').hide();
	$('.div_dialog').hide();
	$('.div_button').tipsy({gravity: 'nw'});
	$('.div_tools').tipsy({gravity: 'nw'});

	$('#refdate_from').datepick({ dateFormat: 'dd.mm.yyyy', pickerClass: 'calendar' });
	$('#refdate_to').datepick({ dateFormat: 'dd.mm.yyyy', pickerClass: 'calendar' });

	$(".div_button").click(function() {
		if(ui[this.id].isOpen)
			ui[this.id].close();					
		else {			
			closeAllInterfacesQuick();
			ui[this.id].open(animationSpeed);									
		}
	});

	$("#map_tools_create").click(function() {		
		polygonCreate.enable();
	});			

	$("#map_tools_edit").click(function() {								
		$('#map_tools_edit_menu').show(animationSpeed);
		polygonEdit.enable();
	});		

	$("#map_tools_edit_save").click(function() {							
		$('#map_tools_edit_menu').hide(animationSpeed);
		polygonEdit.save();
		polygonEdit.disable();
	});		

	$("#map_tools_edit_cancel").click(function() {							
		$('#map_tools_edit_menu').hide(animationSpeed);
		polygonEdit.revertLayers();
		polygonEdit.disable();
	});			

	$("#map_tools_delete").click(function() {											
		$('#map_tools_delete_menu').show(animationSpeed);
		polygonDelete.enable();
	});			

	$("#map_tools_delete_save").click(function() {							
		$('#map_tools_delete_menu').hide(animationSpeed);
		//polygonDelete.save();
		polygonDelete.disable();
	});		

	$("#map_tools_delete_cancel").click(function() {							
		$('#map_tools_delete_menu').hide(animationSpeed);
		polygonDelete.revertLayers();
		polygonDelete.disable();
	});		

	$("#map_tools_search").click(function() {							
		$('#map_dialog_search').toggle(animationSpeed, function () {
			if($('#map_dialog_search').is(':hidden'))
				updateMap();
		});
	});		
});    	

$(window).load(function() {

	ui["map_button"] = new CreateInterface($("#map_menu"), $("#map_content"), function() { map.invalidateSize(false); });
	ui["summary_button"] = new CreateInterface($("#summary_menu"), $("#summary_content"));
	ui["details_button"] = new CreateInterface($("#details_menu"), $("#details_content"));			
	ui["account_button"] = new CreateInterface($("#account_menu"), $("#account_content"));			

	$.getJSON("/api_get_sample_types", function(data) {
		$('#sample_select').empty();
		var output = [];
		output.push('<option value=""></option>');
		$.each(data, function(key, value) {
			output.push('<option value="'+ value +'">'+ value +'</option>');
		});
		$('#sample_select').html(output.join(''));
	});
});

$(window).resize( function(e) { 
	for(k in ui) ui[k].resize(); 
});		

