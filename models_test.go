package gphoto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAlbumlResponseAlbums(t *testing.T) {
	t.Run("Have 1 album", func(t *testing.T) {
		s := `[["wrb.fr","Z5xsfc","[[[\"AF1QipN_G84JQnuyofD4YBpbtWHCTlqrNdbTg823uRQZ\",[\"https://lh3.googleusercontent.com/ARRuDgTKczB7FHobmJpWaGJVles36iY_U5NxxchYlVvmzT9V4sXhZJ-QsSN_TAaoAhxZUydvpJAh6gQWGUteRhs-ukdREIPcLEOSAteXiPB99jZUKMvksXioE8nfSohCuhXSJfU6IoHUVnKi0mSKw-GuiBkd0dZYR2YuLNR5Ybw7j4iuZx0RnOxtwN-MxKwXOmwRWeNk3V9KN6aLfgmPY1KUS7T_Vg2fHGUYgOoZ9CFlua8ifh7VjeNvOmZQBZZfYPckWut9TYSMeityaoixyqYay3siPIurclxlLIzPlOsBug88bnKSeQl3C12RYlb1qSMavDMU4Ajzc9pmSxrJHgpycaqYYcP20am2zVkXO26cbvFBPOTiZc86u-c9GCCfxW4ZvQubkWyHFBBO5M9mvlwyvh1feD0cH_gZjOvq7qE8DFbOaVNdjdRmpv3Y-8Q3sqnHFXPUV7N-gz4rq437B6vE_wgwbWSV_VtgEHpD9iGgMYLBTHT2HRyJ606RP84tLaOw7PhK-gXO8qlJdBHS7DzNsb9j3JtYk_hGDvcuXa4W2tGcBd38CvW7kOk-PJBdOna_KW45JCBqvEJq1kptMS6GxGuyDARhLoS7q2kJELSsLAn0OTd8kEqo555pQcSwEjS3Nzgz9yi9yZQ71wdPIwEjPOjhUrseBFgsAeD7hTJrkHR3YJmjnR-6VX6hSJDnU4DGBywFAkL-GZx1a36CbOPL\",1280,720,null,null,null,null,null,null,[9938222]],null,null,null,null,null,[[3],[4],[5],[6],[8],[21],[10],[11],[13],[12],[16],[19],[22],[23],[34,false,true]],null,null,null,null,null,null,null,{\"72930366\":[1,\"FUAKEHREKKKKKKKKKKKKKKKKKKK\",[1629549478000,1629549478000,null,null,1629551887000,[1629549478000,25200000],[1629549478000,25200000],null,null,1629552023283],1,null,null,[19999]]}]],null,[1]]",null,null,null,"1"],["di",191],["af.httprm",190,"7128235394984688870",20]]`
		r := NewAlbumlResponse(s)
		albums, err := r.Albums()
		require.NoError(t, err)
		assert.Len(t, albums, 1)
		for _, album := range albums {
			assert.NotEmpty(t, album.ID)
			assert.NotEmpty(t, album.Name)
		}
	})

	t.Run("Have 0 album", func(t *testing.T) {
		s := `[["wrb.fr","Z5xsfc","[null,null,[1]]",null,null,null,"3"],["di",111],["af.httprm",110,"-4701098615363376220",24]]`
		r := NewAlbumlResponse(s)
		albums, err := r.Albums()
		require.NoError(t, err)
		assert.Len(t, albums, 0)
	})
}
