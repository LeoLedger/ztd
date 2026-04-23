package parser

import (
	"context"
	"strings"
	"time"

	"github.com/your-name/address-parse/internal/model"
)

// cityDistricts maps each city (value from result.City) to its set of valid districts.
// Keys are normalized city names; values are sets of normalized district names (no suffix).
var cityDistricts = map[string]map[string]bool{
	// Guangdong
	"深圳市": {
		"南山区": true, "福田区": true, "罗湖区": true, "盐田区": true,
		"宝安区": true, "龙岗区": true, "龙华区": true, "坪山区": true,
		"光明区": true, "大鹏新区": true,
	},
	"广州市": {
		"天河区": true, "越秀区": true, "荔湾区": true, "海珠区": true,
		"白云区": true, "黄埔区": true, "番禺区": true, "花都区": true,
		"南沙区": true, "增城区": true, "从化区": true,
	},
	"东莞市": {
		"莞城街道": true, "石龙镇": true, "虎门镇": true, "东城街道": true,
		"万江街道": true, "南城街道": true, "中堂镇": true, "望牛墩镇": true,
		"麻涌镇": true, "石碣镇": true, "高埗镇": true, "洪梅镇": true,
		"道滘镇": true, "厚街镇": true, "沙田镇": true, "长安镇": true,
		"寮步镇": true, "大岭山镇": true, "大朗镇": true, "黄江镇": true,
		"樟木头镇": true, "凤岗镇": true, "塘厦镇": true, "谢岗镇": true,
		"清溪镇": true, "常平镇": true, "桥头镇": true, "横沥镇": true,
		"东坑镇": true, "企石镇": true, "石排镇": true, "茶山镇": true,
		"松山湖": true,
	},
	"佛山市": {
		"禅城区": true, "南海区": true, "顺德区": true, "三水区": true, "高明区": true,
	},
	"惠州市": {
		"惠城区": true, "惠阳区": true, "惠东县": true, "博罗县": true, "龙门县": true,
	},
	"珠海市": {
		"香洲区": true, "斗门区": true, "金湾区": true,
	},
	"中山市": {
		"石岐街道": true, "东区街道": true, "西区街道": true, "南区街道": true,
		"中山港街道": true, "五桂山街道": true,
		"小榄镇": true, "古镇镇": true, "横栏镇": true, "港口镇": true,
		"沙溪镇": true, "大涌镇": true, "黄圃镇": true, "南头镇": true,
		"东凤镇": true, "阜沙镇": true, "三角镇": true, "民众镇": true,
		"南朗镇": true, "三乡镇": true, "板芙镇": true, "神湾镇": true, "坦洲镇": true,
	},
	"汕头市": {
		"金平区": true, "龙湖区": true, "澄海区": true, "濠江区": true,
		"潮阳区": true, "潮南区": true, "南澳县": true,
	},
	"湛江市": {
		"赤坎区": true, "霞山区": true, "坡头区": true, "麻章区": true,
		"徐闻县": true, "廉江市": true, "雷州市": true, "吴川市": true, "遂溪县": true,
	},
	"江门市": {
		"蓬江区": true, "江海区": true, "新会区": true, "台山市": true,
		"开平市": true, "鹤山市": true, "恩平市": true,
	},
	"茂名市": {
		"茂南区": true, "电白区": true, "高州市": true, "化州市": true, "信宜市": true,
	},
	"肇庆市": {
		"端州区": true, "鼎湖区": true, "高要区": true, "四会市": true,
		"广宁县": true, "怀集县": true, "封开县": true, "德庆县": true,
	},
	"梅州市": {
		"梅江区": true, "梅县区": true, "兴宁市": true, "大埔县": true,
		"丰顺县": true, "五华县": true, "平远县": true, "蕉岭县": true,
	},
	"汕尾市": {
		"城区": true, "海丰县": true, "陆丰市": true, "陆河县": true,
	},
	"河源市": {
		"源城区": true, "紫金县": true, "龙川县": true, "连平县": true,
		"和平县": true, "东源县": true,
	},
	"阳江市": {
		"江城区": true, "阳东区": true, "阳西县": true, "阳春市": true,
	},
	"清远市": {
		"清城区": true, "清新区": true, "英德市": true, "连州市": true,
		"佛冈县": true, "阳山县": true, "连山壮族瑶族自治县": true, "连南瑶族自治县": true,
	},
	"韶关市": {
		"浈江区": true, "武江区": true, "曲江区": true, "乐昌市": true,
		"南雄市": true, "始兴县": true, "仁化县": true, "翁源县": true,
		"新丰县": true, "乳源瑶族自治县": true,
	},
	"揭阳市": {
		"榕城区": true, "揭东区": true, "普宁市": true,
		"揭西县": true, "惠来县": true,
	},
	"潮州市": {
		"湘桥区": true, "潮安区": true, "饶平县": true,
	},
	"云浮市": {
		"云城区": true, "云安区": true, "新兴县": true, "郁南县": true, "罗定市": true,
	},

	// Zhejiang
	"杭州市": {
		"上城区": true, "拱墅区": true, "西湖区": true, "滨江区": true,
		"萧山区": true, "余杭区": true, "临平区": true, "钱塘区": true,
		"富阳区": true, "临安区": true, "桐庐县": true, "淳安县": true, "建德市": true,
	},
	"宁波市": {
		"海曙区": true, "江北区": true, "北仑区": true, "镇海区": true,
		"鄞州区": true, "奉化区": true, "余姚市": true, "慈溪市": true,
		"象山县": true, "宁海县": true,
	},
	"温州市": {
		"鹿城区": true, "龙湾区": true, "瓯海区": true, "洞头区": true,
		"瑞安市": true, "乐清市": true, "龙港市": true,
		"永嘉县": true, "平阳县": true, "苍南县": true, "文成县": true, "泰顺县": true,
	},
	"嘉兴市": {
		"南湖区": true, "秀洲区": true, "海宁市": true, "平湖市": true,
		"桐乡市": true, "嘉善县": true, "海盐县": true,
	},
	"湖州市": {
		"吴兴区": true, "南浔区": true, "德清县": true, "长兴县": true, "安吉县": true,
	},
	"绍兴市": {
		"越城区": true, "柯桥区": true, "上虞区": true,
		"诸暨市": true, "嵊州市": true, "新昌县": true,
	},
	"金华市": {
		"婺城区": true, "金东区": true, "兰溪市": true, "义乌市": true,
		"东阳市": true, "永康市": true, "武义县": true, "浦江县": true, "磐安县": true,
	},
	"衢州市": {
		"柯城区": true, "衢江区": true, "江山市": true,
		"常山县": true, "开化县": true, "龙游县": true,
	},
	"舟山市": {
		"定海区": true, "普陀区": true, "岱山县": true, "嵊泗县": true,
	},
	"台州市": {
		"椒江区": true, "黄岩区": true, "路桥区": true,
		"临海市": true, "温岭市": true, "玉环市": true,
		"三门县": true, "天台县": true, "仙居县": true,
	},
	"丽水市": {
		"莲都区": true, "龙泉市": true, "青田县": true, "缙云县": true,
		"遂昌县": true, "松阳县": true, "云和县": true,
		"庆元县": true, "景宁畲族自治县": true,
	},

	// Jiangsu
	"南京市": {
		"玄武区": true, "秦淮区": true, "建邺区": true, "鼓楼区": true,
		"浦口区": true, "栖霞区": true, "雨花台区": true, "江宁区": true,
		"六合区": true, "溧水区": true, "高淳区": true,
	},
	"苏州市": {
		"姑苏区": true, "虎丘区": true, "吴中区": true, "相城区": true,
		"工业园区": true, "高新区": true, "吴江区": true,
		"常熟市": true, "张家港市": true, "昆山市": true, "太仓市": true,
	},
	"无锡市": {
		"梁溪区": true, "锡山区": true, "惠山区": true, "滨湖区": true,
		"新吴区": true, "江阴市": true, "宜兴市": true,
	},
	"常州市": {
		"天宁区": true, "钟楼区": true, "新北区": true, "武进区": true,
		"溧阳市": true, "金坛区": true,
	},
	"镇江市": {
		"京口区": true, "润州区": true, "丹徒区": true,
		"丹阳市": true, "扬中市": true, "句容市": true,
	},
	"扬州市": {
		"广陵区": true, "邗江区": true, "江都区": true,
		"仪征市": true, "高邮市": true, "宝应县": true,
	},
	"泰州市": {
		"海陵区": true, "高港区": true, "姜堰区": true,
		"兴化市": true, "靖江市": true, "泰兴市": true,
	},
	"南通市": {
		"崇川区": true, "通州区": true, "海门区": true,
		"启东市": true, "如皋市": true, "海安市": true, "如东县": true,
	},
	"盐城市": {
		"亭湖区": true, "盐都区": true, "大丰区": true,
		"东台市": true, "响水县": true, "滨海县": true,
		"阜宁县": true, "射阳县": true, "建湖县": true,
	},
	"淮安市": {
		"清江浦区": true, "淮安区": true, "淮阴区": true, "洪泽区": true,
		"涟水县": true, "盱眙县": true, "金湖县": true,
	},
	"连云港市": {
		"连云区": true, "海州区": true, "赣榆区": true,
		"东海县": true, "灌云县": true, "灌南县": true,
	},
	"徐州市": {
		"鼓楼区": true, "云龙区": true, "贾汪区": true, "泉山区": true,
		"铜山区": true, "新沂市": true, "邳州市": true, "丰县": true, "沛县": true, "睢宁县": true,
	},
	"宿迁市": {
		"宿城区": true, "宿豫区": true, "沭阳县": true, "泗阳县": true, "泗洪县": true,
	},

	// Beijing
	"北京市": {
		"东城区": true, "西城区": true, "朝阳区": true, "丰台区": true,
		"石景山区": true, "海淀区": true, "门头沟区": true, "房山区": true,
		"通州区": true, "顺义区": true, "昌平区": true, "大兴区": true,
		"怀柔区": true, "平谷区": true, "密云区": true, "延庆区": true,
	},

	// Shanghai
	"上海市": {
		"黄浦区": true, "徐汇区": true, "长宁区": true, "静安区": true,
		"普陀区": true, "虹口区": true, "杨浦区": true, "闵行区": true,
		"宝山区": true, "嘉定区": true, "浦东新区": true, "金山区": true,
		"松江区": true, "青浦区": true, "奉贤区": true, "崇明区": true,
	},

	// Chongqing
	"重庆市": {
		"万州区": true, "渝中区": true, "江北区": true, "沙坪坝区": true,
		"九龙坡区": true, "南岸区": true, "北碚区": true, "渝北区": true,
		"巴南区": true, "涪陵区": true, "长寿区": true, "璧山区": true,
		"合川区": true, "永川区": true, "江津区": true, "綦江区": true,
		"大足区": true, "铜梁区": true, "潼南区": true, "荣昌区": true,
		"开州区": true, "梁平区": true,
	},

	// Sichuan
	"成都市": {
		"锦江区": true, "青羊区": true, "金牛区": true, "武侯区": true,
		"成华区": true, "龙泉驿区": true, "青白江区": true, "新都区": true,
		"温江区": true, "双流区": true, "郫都区": true,
		"新津区": true, "简阳市": true, "都江堰市": true, "彭州市": true,
		"邛崃市": true, "崇州市": true, "大邑县": true, "蒲江县": true,
	},
	"绵阳市": {
		"涪城区": true, "游仙区": true, "安州区": true,
		"江油市": true, "三台县": true, "盐亭县": true, "梓潼县": true,
		"平武县": true, "北川羌族自治县": true,
	},
	"德阳市": {
		"旌阳区": true, "罗江区": true, "广汉市": true, "什邡市": true, "绵竹市": true, "中江县": true,
	},
	"宜宾市": {
		"翠屏区": true, "南溪区": true, "叙州区": true,
		"江安县": true, "长宁县": true, "高县": true, "筠连县": true,
		"珙县": true, "兴文县": true, "屏山县": true,
	},
	"南充市": {
		"顺庆区": true, "高坪区": true, "嘉陵区": true,
		"阆中市": true, "南部县": true, "营山县": true,
		"蓬安县": true, "仪陇县": true, "西充县": true,
	},

	// Hubei
	"武汉市": {
		"江岸区": true, "江汉区": true, "硚口区": true, "汉阳区": true,
		"武昌区": true, "青山区": true, "洪山区": true, "东西湖区": true,
		"汉南区": true, "蔡甸区": true, "江夏区": true, "黄陂区": true, "新洲区": true,
	},
	"襄阳市": {
		"襄城区": true, "樊城区": true, "襄州区": true,
		"枣阳市": true, "宜城市": true, "老河口市": true,
		"南漳县": true, "谷城县": true, "保康县": true,
	},
	"宜昌市": {
		"西陵区": true, "伍家岗区": true, "点军区": true, "猇亭区": true,
		"夷陵区": true, "宜都市": true, "当阳市": true, "枝江市": true,
		"远安县": true, "兴山县": true, "秭归县": true, "长阳土家族自治县": true, "五峰土家族自治县": true,
	},

	// Hunan
	"长沙市": {
		"芙蓉区": true, "天心区": true, "岳麓区": true, "开福区": true,
		"雨花区": true, "望城区": true, "长沙县": true,
		"浏阳市": true, "宁乡市": true,
	},

	// Anhui
	"合肥市": {
		"瑶海区": true, "庐阳区": true, "蜀山区": true, "包河区": true,
		"庐江县": true, "肥东县": true, "肥西县": true,
		"巢湖市": true,
	},

	// Fujian
	"福州市": {
		"鼓楼区": true, "台江区": true, "仓山区": true, "马尾区": true,
		"晋安区": true, "长乐区": true,
		"闽侯县": true, "连江县": true, "罗源县": true, "闽清县": true,
		"永泰县": true, "平潭县": true, "福清市": true,
	},
	"厦门市": {
		"思明区": true, "海沧区": true, "湖里区": true, "集美区": true,
		"同安区": true, "翔安区": true,
	},
	"泉州市": {
		"鲤城区": true, "丰泽区": true, "洛江区": true, "泉港区": true,
		"惠安县": true, "安溪县": true, "永春县": true, "德化县": true,
		"石狮市": true, "晋江市": true, "南安市": true,
	},

	// Shandong
	"济南市": {
		"历下区": true, "市中区": true, "槐荫区": true, "天桥区": true,
		"历城区": true, "长清区": true, "章丘区": true, "济阳区": true,
		"平阴县": true, "商河县": true,
	},
	"青岛市": {
		"市南区": true, "市北区": true, "黄岛区": true, "崂山区": true,
		"李沧区": true, "城阳区": true, "即墨区": true,
		"胶州市": true, "平度市": true, "莱西市": true,
	},

	// Henan
	"郑州市": {
		"中原区": true, "二七区": true, "管城回族区": true, "金水区": true,
		"上街区": true, "惠济区": true, "中牟县": true,
		"巩义市": true, "荥阳市": true, "新密市": true, "新郑市": true, "登封市": true,
	},
	"洛阳市": {
		"老城区": true, "西工区": true, "瀍河回族区": true, "涧西区": true,
		"偃师区": true, "孟津区": true, "洛龙区": true,
		"新安县": true, "栾川县": true, "嵩县": true, "汝阳县": true,
		"宜阳县": true, "洛宁县": true, "伊川县": true,
	},
}

// cityNormMaps maps common city abbreviations/partial names to their standard full names.
// Used to normalize city names from the rule engine (e.g. "深圳" → "深圳市").
var cityNormMaps = map[string]string{
	"深圳": "深圳市", "广州": "广州市", "东莞": "东莞市", "佛山": "佛山市",
	"惠州": "惠州市", "珠海": "珠海市", "中山": "中山市", "汕头": "汕头市",
	"湛江": "湛江市", "江门": "江门市", "茂名": "茂名市", "肇庆": "肇庆市",
	"梅州": "梅州市", "汕尾": "汕尾市", "河源": "河源市", "阳江": "阳江市",
	"清远": "清远市", "韶关": "韶关市", "揭阳": "揭阳市", "潮州": "潮州市",
	"云浮": "云浮市",
	"杭州": "杭州市", "宁波": "宁波市", "温州": "温州市", "嘉兴": "嘉兴市",
	"湖州": "湖州市", "绍兴": "绍兴市", "金华": "金华市", "衢州": "衢州市",
	"舟山": "舟山市", "台州": "台州市", "丽水": "丽水市",
	"南京": "南京市", "苏州": "苏州市", "无锡": "无锡市", "常州": "常州市",
	"镇江": "镇江市", "扬州": "扬州市", "泰州": "泰州市", "南通": "南通市",
	"盐城": "盐城市", "淮安": "淮安市", "连云港": "连云港市", "徐州": "徐州市",
	"宿迁": "宿迁市",
	"北京": "北京市",
	"上海": "上海市",
	"天津": "天津市",
	"重庆": "重庆市",
	"武汉": "武汉市", "襄阳": "襄阳市", "宜昌": "宜昌市",
	"长沙": "长沙市",
	"合肥": "合肥市",
	"福州": "福州市", "厦门": "厦门市", "泉州": "泉州市",
	"济南": "济南市", "青岛": "青岛市",
	"郑州": "郑州市", "洛阳": "洛阳市",
	"成都": "成都市",
	"南昌": "南昌市",
}

// NormalizeCity returns the standard city name, adding "市" suffix if needed.
func NormalizeCity(city string) string {
	if city == "" {
		return ""
	}
	// Already has "市" suffix.
	if strings.HasSuffix(city, "市") {
		return city
	}
	// Look up abbreviation.
	if full, ok := cityNormMaps[city]; ok {
		return full
	}
	return city
}

// streetToDistrict maps known subdistrict/street names to their district.
// These are used both for correction (when district is wrong) and auto-fill (when district is missing).
// Key format: "城市:街道名" → "所属区"
var streetToDistrict = map[string]string{
	// Shenzhen subdistricts
	"深圳市:观湖街道":    "龙华区",
	"深圳市:观澜街道":    "龙华区",
	"深圳市:龙华街道":    "龙华区",
	"深圳市:民治街道":    "龙华区",
	"深圳市:大浪街道":    "龙华区",
	"深圳市:福城街道":    "龙华区",
	"深圳市:民乐街道":    "龙华区",
	"深圳市:清华街道":    "龙华区",
	"深圳市:冲之大道":    "龙华区",
	"深圳市:梅观路":      "龙华区",
	"深圳市:富士康":      "龙华区",
	"深圳市:锦绣科学园":  "龙华区",
	"深圳市:富士康科技园": "龙华区",
	"深圳市:高尔夫":      "龙华区",
	"深圳市:鹭湖":        "龙华区",
	"深圳市:横坑":        "龙华区",
	"深圳市:牛湖":        "龙华区",
	"深圳市:桂花":        "龙华区",
	"深圳市:松元厦":      "龙华区",
	"深圳市:谭罗":        "龙华区",
	"深圳市:库坑":        "龙华区",
	"深圳市:桔塘":        "龙华区",
	"深圳市:大和":        "龙华区",
	"深圳市:新石湖":      "龙华区",
	"深圳市:黎光":        "龙华区",
	"深圳市:龙华中心":     "龙华区",
	"深圳市:梅观高速":     "龙华区",
	"深圳市:龙华汽车站":   "龙华区",

	"深圳市:盐田街道":    "盐田区",
	"深圳市:海山街道":    "盐田区",
	"深圳市:梅沙街道":    "盐田区",
	"深圳市:沙头角街道":  "盐田区",
	"深圳市:盐田港":      "盐田区",

	"深圳市:南山区":      "南山区",
	"深圳市:粤海街道":    "南山区",
	"深圳市:蛇口街道":    "南山区",
	"深圳市:招商街道":    "南山区",
	"深圳市:桃源街道":    "南山区",
	"深圳市:西丽街道":    "南山区",
	"深圳市:沙河街道":    "南山区",
	"深圳市:科技园":     "南山区",
	"深圳市:深圳大学城":  "南山区",
	"深圳市:深圳软件园":  "南山区",
	"深圳市:深圳湾":      "南山区",
	"深圳市:后海":        "南山区",
	"深圳市:前海":        "南山区",
	"深圳市:前海深港现代服务业合作区": "南山区",

	"深圳市:福田街道":    "福田区",
	"深圳市:南园街道":    "福田区",
	"深圳市:园岭街道":    "福田区",
	"深圳市:沙头街道":    "福田区",
	"深圳市:梅林街道":    "福田区",
	"深圳市:华富街道":    "福田区",
	"深圳市:莲花街道":    "福田区",
	"深圳市:华强北街道":  "福田区",
	"深圳市:福田CBD":     "福田区",
	"深圳市:深圳CBD":     "福田区",
	"深圳市:市民中心":    "福田区",
	"深圳市:深圳证券交易所": "福田区",

	"深圳市:桂园街道":    "罗湖区",
	"深圳市:南湖街道":    "罗湖区",
	"深圳市:笋岗街道":    "罗湖区",
	"深圳市:清水河街道":  "罗湖区",
	"深圳市:翠竹街道":    "罗湖区",
	"深圳市:东门街道":    "罗湖区",
	"深圳市:黄贝街道":    "罗湖区",
	"深圳市:莲塘街道":    "罗湖区",
	"深圳市:东晓街道":    "罗湖区",
	"深圳市:罗湖口岸":    "罗湖区",
	"深圳市:国贸":        "罗湖区",

	"深圳市:新安街道":    "宝安区",
	"深圳市:西乡街道":    "宝安区",
	"深圳市:航城街道":    "宝安区",
	"深圳市:福永街道":    "宝安区",
	"深圳市:福海街道":    "宝安区",
	"深圳市:沙井街道":    "宝安区",
	"深圳市:新桥街道":    "宝安区",
	"深圳市:松岗街道":    "宝安区",
	"深圳市:燕罗街道":    "宝安区",
	"深圳市:石岩街道":    "宝安区",
	"深圳市:宝安中心区":   "宝安区",
	"深圳市:深圳机场":    "宝安区",
	"深圳市:国际会展中心": "宝安区",

	"深圳市:平湖街道":    "龙岗区",
	"深圳市:布吉街道":    "龙岗区",
	"深圳市:吉华街道":    "龙岗区",
	"深圳市:坂田街道":    "龙岗区",
	"深圳市:南湾街道":    "龙岗区",
	"深圳市:横岗街道":    "龙岗区",
	"深圳市:园山街道":    "龙岗区",
	"深圳市:龙城街道":    "龙岗区",
	"深圳市:龙岗街道":    "龙岗区",
	"深圳市:坪地街道":    "龙岗区",
	"深圳市:大运新城":    "龙岗区",
	"深圳市:深圳北理莫斯科大学": "龙岗区",

	"深圳市:坪山街道":    "坪山区",
	"深圳市:坑梓街道":    "坪山区",
	"深圳市:坪山高新区":  "坪山区",

	"深圳市:光明街道":    "光明区",
	"深圳市:公明街道":    "光明区",
	"深圳市:新湖街道":    "光明区",
	"深圳市:凤凰街道":    "光明区",
	"深圳市:玉塘街道":    "光明区",
	"深圳市:马田街道":    "光明区",
	"深圳市:中山大学":    "光明区",
	"深圳市:光明科学城":  "光明区",

	"深圳市:盐田保税区":  "盐田区",

	"深圳市:葵涌街道":    "大鹏新区",
	"深圳市:大鹏街道":    "大鹏新区",
	"深圳市:南澳街道":    "大鹏新区",
	"深圳市:大鹏半岛":    "大鹏新区",

	// Guangzhou subdistricts
	"广州市:猎德":       "天河区",
	"广州市:冼村":       "天河区",
	"广州市:天河南":     "天河区",
	"广州市:员村":       "天河区",
	"广州市:棠下":       "天河区",
	"广州市:石牌":       "天河区",
	"广州市:天园":       "天河区",
	"广州市:林和":       "天河区",
	"广州市:林和街道":   "天河区",
	"广州市:员村街道":   "天河区",
	"广州市:冼村街道":   "天河区",
	"广州市:猎德街道":   "天河区",
	"广州市:沙东":       "天河区",
	"广州市:五山":       "天河区",
	"广州市:珠吉":       "天河区",
	"广州市:龙洞":       "天河区",
	"广州市:凤凰":       "天河区",
	"广州市:新塘":       "天河区",
	"广州市:岑村":       "天河区",
	"广州市:科学城":     "黄埔区",
	"广州市:知识城":     "黄埔区",
	"广州市:广州科学城": "黄埔区",
	"广州市:广州知识城": "黄埔区",

	// Beijing subdistricts
	"北京市:望京":       "朝阳区",
	"北京市:三里屯":     "朝阳区",
	"北京市:国贸":        "朝阳区",
	"北京市:CBD":        "朝阳区",
	"北京市:双井":        "朝阳区",
	"北京市:劲松":        "朝阳区",
	"北京市:潘家园":      "朝阳区",
	"北京市:建外":        "朝阳区",
	"北京市:朝外":        "朝阳区",
	"北京市:呼家楼":      "朝阳区",
	"北京市:团结湖":      "朝阳区",
	"北京市:六里屯":      "朝阳区",
	"北京市:麦子店":      "朝阳区",
	"北京市:将台":        "朝阳区",
	"北京市:东湖":        "朝阳区",
	"北京市:大望路":      "朝阳区",

	"北京市:中关村":      "海淀区",
	"北京市:上地":        "海淀区",
	"北京市:五道口":      "海淀区",
	"北京市:清华":        "海淀区",
	"北京市:北大":        "海淀区",
	"北京市:学院路":      "海淀区",
	"北京市:西二旗":      "海淀区",
	"北京市:知春路":      "海淀区",
	"北京市:中关村软件园": "海淀区",
	"北京市:北京航空航天大学": "海淀区",
	"北京市:北京大学":    "海淀区",
	"北京市:清华大学":    "海淀区",
	"北京市:中国人民大学": "海淀区",
	"北京市:北京师范大学": "海淀区",
	"北京市:北京理工大学": "海淀区",
	"北京市:北京外国语大学": "海淀区",

	"北京市:亦庄":        "大兴区",
	"北京市:北京经济技术开发区": "大兴区",
	"北京市:大兴生物医药基地": "大兴区",

	// Shanghai subdistricts
	"上海市:陆家嘴":      "浦东新区",
	"上海市:浦东":        "浦东新区",
	"上海市:张江":        "浦东新区",
	"上海市:金桥":        "浦东新区",
	"上海市:外高桥":      "浦东新区",
	"上海市:前滩":        "浦东新区",
	"上海市:世博":        "浦东新区",
	"上海市:花木":        "浦东新区",
	"上海市:川沙":        "浦东新区",
	"上海市:周浦":        "浦东新区",
	"上海市:康桥":        "浦东新区",
	"上海市:三林":        "浦东新区",
	"上海市:南码头":      "浦东新区",
	"上海市:塘桥":        "浦东新区",
	"上海市:上钢":        "浦东新区",
	"上海市:洋泾":        "浦东新区",
	"上海市:沪东":        "浦东新区",
	"上海市:浦兴":        "浦东新区",
	"上海市:东明":        "浦东新区",
	"上海市:潍坊":        "浦东新区",
	"上海市:张江高科技园区": "浦东新区",
	"上海市:张江科学城":  "浦东新区",
	"上海市:上海张江":    "浦东新区",

	"上海市:徐家汇":      "徐汇区",
	"上海市:漕河泾":      "徐汇区",
	"上海市:虹梅":        "徐汇区",
	"上海市:华泾":        "徐汇区",
	"上海市:长桥":        "徐汇区",
	"上海市:凌云":        "徐汇区",
	"上海市:龙华":        "徐汇区",
	"上海市:天平":        "徐汇区",
	"上海市:湖南":        "徐汇区",
	"上海市:枫林":        "徐汇区",

	"上海市:静安寺":      "静安区",
	"上海市:曹家渡":      "静安区",
	"上海市:江宁路":      "静安区",
	"上海市:石门二路":    "静安区",
	"上海市:南京西路":    "静安区",
	"上海市:北站":        "静安区",
	"上海市:宝山路":      "静安区",
	"上海市:芷江西路":    "静安区",
	"上海市:共和新路":    "静安区",
	"上海市:大宁路":      "静安区",
	"上海市:临汾":        "静安区",
	"上海市:天目西路":    "静安区",
	"上海市:大宁":        "静安区",

	"上海市:五角场":      "杨浦区",
	"上海市:新江湾城":    "杨浦区",
	"上海市:长白":        "杨浦区",
	"上海市:延吉":        "杨浦区",
	"上海市:控江":        "杨浦区",
	"上海市:四平":        "杨浦区",
	"上海市:殷行":        "杨浦区",
	"上海市:大桥":        "杨浦区",
	"上海市:平凉":        "杨浦区",
	"上海市:定海":        "杨浦区",
	"上海市:彭浦":        "杨浦区",

	"上海市:虹桥":        "长宁区",
	"上海市:新华":        "长宁区",
	"上海市:江苏路":      "长宁区",
	"上海市:华阳":        "长宁区",
	"上海市:周家桥":      "长宁区",
	"上海市:天山":        "长宁区",
	"上海市:仙霞":        "长宁区",
	"上海市:程家桥":      "长宁区",
	"上海市:北新泾":      "长宁区",
	"上海市:新泾":        "长宁区",

	// Hangzhou subdistricts
	"杭州市:四季青":      "上城区",
	"杭州市:闸弄口":      "上城区",
	"杭州市:采荷":        "上城区",
	"杭州市:凯旋":        "上城区",
	"杭州市:彭埠":        "上城区",
	"杭州市:笕桥":        "上城区",
	"杭州市:九堡":        "上城区",
	"杭州市:丁桥":        "上城区",
	"杭州市:钱江新城":    "上城区",
	"杭州市:望江":        "上城区",
	"杭州市:紫阳":        "上城区",
	"杭州市:南星":        "上城区",

	"杭州市:西兴":        "滨江区",
	"杭州市:长河":        "滨江区",
	"杭州市:浦沿":        "滨江区",
	"杭州市:物联网小镇":  "滨江区",
	"杭州市:互联网小镇":  "滨江区",
	"杭州市:滨江高新区":  "滨江区",

	"杭州市:翠苑":        "西湖区",
	"杭州市:文新":        "西湖区",
	"杭州市:古荡":        "西湖区",
	"杭州市:留下":        "西湖区",
	"杭州市:转塘":        "西湖区",
	"杭州市:蒋村":        "西湖区",
	"杭州市:三墩":        "西湖区",
	"杭州市:文二":        "西湖区",
	"杭州市:文三":        "西湖区",
	"杭州市:黄龙":        "西湖区",
	"杭州市:云栖小镇":    "西湖区",
	"杭州市:西溪":        "西湖区",
	"杭州市:浙江大学":    "西湖区",
	"杭州市:紫金港":      "西湖区",
	"杭州市:玉泉":        "西湖区",

	"杭州市:白杨":        "钱塘区",
	"杭州市:下沙":        "钱塘区",
	"杭州市:河庄":        "钱塘区",
	"杭州市:义蓬":        "钱塘区",
	"杭州市:新湾":        "钱塘区",
	"杭州市:临江":        "钱塘区",
	"杭州市:前进":        "钱塘区",
	"杭州市:下沙高教园区": "钱塘区",

	"杭州市:南苑":        "临平区",
	"杭州市:东湖":        "临平区",
	"杭州市:星桥":        "临平区",
	"杭州市:乔司":        "临平区",
	"杭州市:运河":        "临平区",
	"杭州市:塘栖":        "临平区",
	"杭州市:崇贤":        "临平区",
	"杭州市:临平新城":    "临平区",
	"杭州市:余杭经济技术开发区": "临平区",

	"杭州市:余杭":        "余杭区",
	"杭州市:仓前":        "余杭区",
	"杭州市:闲林":        "余杭区",
	"杭州市:五常":        "余杭区",
	"杭州市:中泰":        "余杭区",
	"杭州市:瓶窑":        "余杭区",
	"杭州市:良渚":        "余杭区",
	"杭州市:仁和":        "余杭区",
	"杭州市:未来科技城": "余杭区",
	"杭州市:梦想小镇":    "余杭区",
	"杭州市:海创园":      "余杭区",
	"杭州市:阿里巴巴西溪园区": "余杭区",

	"杭州市:北干":        "萧山区",
	"杭州市:蜀山":        "萧山区",
	"杭州市:新塘":        "萧山区",
	"杭州市:宁围":        "萧山区",
	"杭州市:新街":        "萧山区",
	"杭州市:靖江":        "萧山区",
	"杭州市:南阳":        "萧山区",
	"杭州市:河上":        "萧山区",
	"杭州市:浦阳":        "萧山区",
	"杭州市:进化":        "萧山区",
	"杭州市:临浦":        "萧山区",
	"杭州市:瓜沥":        "萧山区",
	"杭州市:衙前":        "萧山区",
	"杭州市:萧山经济技术开发区": "萧山区",
	"杭州市:钱江世纪城":  "萧山区",
	"杭州市:萧山科技城":  "萧山区",
	"杭州市:萧山机场":    "萧山区",

	// Nanjing subdistricts
	"南京市:江宁":        "江宁区",
	"南京市:东山":        "江宁区",
	"南京市:秣陵":        "江宁区",
	"南京市:汤山":        "江宁区",
	"南京市:麒麟":        "江宁区",
	"南京市:南京江宁":    "江宁区",
	"南京市:百家湖":       "江宁区",
	"南京市:九龙湖":       "江宁区",
	"南京市:大学城":       "江宁区",
	"南京市:南京南站":     "雨花台区",

	// Chengdu subdistricts
	"成都市:金融城":      "高新区",
	"成都市:大源":        "高新区",
	"成都市:中和":        "高新区",
	"成都市:石羊":        "高新区",
	"成都市:桂溪":        "高新区",
	"成都市:肖家河":      "高新区",
	"成都市:芳草":        "高新区",
	"成都市:合作":        "高新区",
	"成都市:西园":        "高新区",
	"成都市:成都高新区":  "高新区",
	"成都市:成都天府新区": "天府新区",
	"成都市:天府新区":     "天府新区",
	"成都市:兴隆湖":      "天府新区",
	"成都市:秦皇寺":      "天府新区",
	"成都市:华阳":        "天府新区",

	// Wuhan subdistricts
	"武汉市:光谷":         "洪山区",
	"武汉市:光谷广场":     "洪山区",
	"武汉市:光谷软件园":   "洪山区",
	"武汉市:关山":         "洪山区",
	"武汉市:关山街道":     "洪山区",
	"武汉市:珞瑜":         "洪山区",
	"武汉市:鲁巷":         "洪山区",

	// Huizhou subdistricts
	"惠州市:河南岸街道":  "惠城区",
	"惠州市:江北街道":    "惠城区",
	"惠州市:桥西街道":    "惠城区",
	"惠州市:桥东街道":    "惠城区",
	"惠州市:龙丰街道":    "惠城区",
	"惠州市:江南街道":    "惠城区",
	"惠州市:惠环街道":    "惠城区",
	"惠州市:陈江街道":    "惠城区",
	"惠州市:水口街道":    "惠城区",
	"惠州市:小金口街道":  "惠城区",
	"惠州市:汝湖镇":      "惠城区",
	"惠州市:三栋镇":      "惠城区",
	"惠州市:马安镇":      "惠城区",
	"惠州市:横沥镇":      "惠城区",
	"惠州市:芦洲镇":      "惠城区",
	"惠州市:大亚湾":      "惠阳区",
	"惠州市:澳头街道":    "惠阳区",
	"惠州市:西区街道":    "惠阳区",
	"惠州市:霞涌街道":    "惠阳区",
	"惠州市:淡水街道":    "惠阳区",
	"惠州市:秋长街道":    "惠阳区",
	"惠州市:三和街道":    "惠阳区",
	"惠州市:新圩镇":      "惠阳区",
	"惠州市:镇隆镇":      "惠阳区",
	"惠州市:沙田镇":      "惠阳区",
	"惠州市:永湖镇":      "惠阳区",
	"惠州市:良井镇":      "惠阳区",
	"惠州市:平潭镇":      "惠阳区",

	// Dongguan subdistricts
	"东莞市:南城街道":    "南城区",
	"东莞市:东城街道":    "东城区",
	"东莞市:莞城街道":    "莞城区",
	"东莞市:万江街道":    "万江区",
	"东莞市:虎门镇":      "滨海片区",
	"东莞市:长安镇":      "滨海片区",
	"东莞市:厚街镇":      "滨海片区",
	"东莞市:松山湖":      "松山湖功能区",
	"东莞市:大朗镇":      "松山湖功能区",
	"东莞市:大岭山镇":    "松山湖功能区",
}

// DistrictValidator validates parsed districts and fills in missing ones.
type DistrictValidator struct {
	geocoder *AMapGeocoder
}

// NewDistrictValidator creates a new district validator without geocoding support.
// Use NewDistrictValidatorWithGeocoder for production.
func NewDistrictValidator() *DistrictValidator {
	return &DistrictValidator{}
}

// NewDistrictValidatorWithGeocoder creates a validator backed by a geocoder
// that is used as a last-resort fallback when street→district lookup fails.
func NewDistrictValidatorWithGeocoder(geocoder *AMapGeocoder) *DistrictValidator {
	return &DistrictValidator{geocoder: geocoder}
}

// ValidateAndCorrect checks whether the parsed district is valid for the given city.
// It also verifies street→district consistency even when the district name is valid.
// Returns a correction if the district is invalid OR the street contradicts it.
func (v *DistrictValidator) ValidateAndCorrect(city, district string, street, detail string) *model.DistrictCorrection {
	if district == "" || city == "" {
		return nil
	}

	city = NormalizeCity(city)

	normalizedDistrict := normalizeDistrict(district)

	// Guard: detect "street name used as district" even when the stripped name
	// happens to be a substring of a valid district entry.
	// Example: district="河南岸街道" → strips to "河南岸" → matches cityDistricts key
	// "惠城区" (because stripSuffix returns "惠城"), but the VALUE is a street name.
	if corrected := v.detectStreetNameMasqueradingAsDistrict(city, district); corrected != "" {
		return &model.DistrictCorrection{
			InputDistrict:     district,
			CorrectedDistrict: corrected,
			Reason:           "区划字段填写的是街道/社区名称而非区县名，已纠正为" + corrected,
			CorrectionType:   "invalid_district",
		}
	}

	// Try to find a street-based district inference.
	streetBased := v.findCorrectDistrict(city, street, detail)

	// Check if district is valid for this city.
	validDistricts, cityExists := cityDistricts[city]
	districtValid := cityExists && validDistricts[normalizedDistrict]

	if districtValid && streetBased != "" {
		// District name is valid, but street may contradict it. Verify consistency.
		if streetBased != normalizedDistrict {
			return &model.DistrictCorrection{
				InputDistrict:     district,
				CorrectedDistrict: streetBased,
				Reason:           "街道与区县不一致，根据街道推断应属于" + streetBased,
				CorrectionType:   "street_mismatch",
			}
		}
		// District is valid AND consistent with street — no correction needed.
		return nil
	}

	if districtValid {
		// District name is valid; no correction needed.
		return nil
	}

	// District is invalid. Try to find the correct district.
	if streetBased != "" {
		return &model.DistrictCorrection{
			InputDistrict:     district,
			CorrectedDistrict: streetBased,
			Reason:           "区划不在该城市范围内，根据街道/路名推断应属于" + streetBased,
			CorrectionType:   "invalid_district",
		}
	}

	// No street info available. Try to find correction by analyzing the district name itself
	// (e.g. "惠城区河南岸街道金湖社区" contains "河南岸街道" → 惠城区).
	if nameBased := v.findCorrectDistrictByName(city, district); nameBased != "" {
		return &model.DistrictCorrection{
			InputDistrict:     district,
			CorrectedDistrict: nameBased,
			Reason:           "根据区县名称内含街道/社区推断，应属于" + nameBased,
			CorrectionType:   "invalid_district",
		}
	}

	return &model.DistrictCorrection{
		InputDistrict:     district,
		CorrectedDistrict: "",
		Reason:            "区划不在该城市范围内",
		CorrectionType:   "invalid_district",
	}
}

// detectStreetNameMasqueradingAsDistrict checks whether district is a known street
// or community name (from streetToDistrict) that the LLM may have incorrectly
// placed in the district field. Returns the correct district or "".
func (v *DistrictValidator) detectStreetNameMasqueradingAsDistrict(city, district string) string {
	if city == "" || district == "" {
		return ""
	}

	// Direct key lookup: "惠州市:河南岸街道" or "深圳市:南山区"
	if target, ok := streetToDistrict[city+":"+district]; ok {
		// Only correct if streetToDistrict maps to a DIFFERENT district.
		// If the target is the same as the input district (e.g. "深圳市:南山区"→"南山区"),
		// then the value is a real district name, not a street name — no correction.
		if target != district {
			return target
		}
	}

	// Also check with the "街道/镇/乡/社区/路/大道/巷" suffix stripped.
	normalized := normalizeStreet(district)
	if normalized != district {
		if target, ok := streetToDistrict[city+":"+normalized]; ok && target != district {
			return target
		}
	}

	// Substring scan: only for values that are clearly street-level
	// (ending in 街道/镇/乡/社区/路/etc), NOT real district names (ending in 区/县/市/etc).
	// This avoids false-positives like "南山区" containing the street key "南山".
	isRealDistrict := strings.HasSuffix(district, "区") ||
		strings.HasSuffix(district, "县") ||
		strings.HasSuffix(district, "市") ||
		strings.HasSuffix(district, "旗") ||
		strings.HasSuffix(district, "州")
	if isRealDistrict {
		return ""
	}

	for key, target := range streetToDistrict {
		if !strings.HasPrefix(key, city+":") {
			continue
		}
		streetName := strings.TrimPrefix(key, city+":")
		if strings.Contains(district, streetName) && target != district {
			return target
		}
	}

	return ""
}

// AutoFillDistrict attempts to infer the district when it is missing or unrecognized.
// It also kicks in when the parsed district is invalid — treating the capture as a miss.
func (v *DistrictValidator) AutoFillDistrict(ctx context.Context, city, district string, street, detail string) *model.DistrictAutoFill {
	return v.AutoFillDistrictWithOriginal(ctx, city, district, street, detail, "")
}

// AutoFillDistrictWithOriginal is the same as AutoFillDistrict but also searches originalText
// for district/street hints when the detail field is insufficient.
// This handles cases like "惠州市河南岸街道金湖社区张屋山一巷二号" where the detail field
// only contains "张屋山一巷二号" but "河南岸街道" (which maps to a district) is elsewhere.
func (v *DistrictValidator) AutoFillDistrictWithOriginal(ctx context.Context, city, district, street, detail, originalText string) *model.DistrictAutoFill {
	if city == "" {
		return nil
	}
	city = NormalizeCity(city)

	// Check if the captured district is actually valid for this city.
	// If it is, no auto-fill needed.
	if district != "" {
		normalized := normalizeDistrict(district)
		if valid, ok := cityDistricts[city]; ok && valid[normalized] {
			return nil
		}
	}

	// Try to infer from street name.
	if inferred := v.inferFromStreet(city, street); inferred != "" {
		return &model.DistrictAutoFill{
			InferredDistrict: inferred,
			InferenceSource:  "street_name",
		}
	}

	// Try to infer from detail address fragments.
	if inferred := v.inferFromDetail(city, detail); inferred != "" {
		return &model.DistrictAutoFill{
			InferredDistrict: inferred,
			InferenceSource:  "detail_address",
		}
	}

	// Try to infer from the full original text.
	// This handles cases where the detail field is too narrow (e.g. just a house number)
	// but the original address contains street/community names that hint at the district.
	if inferred := v.inferFromOriginalText(city, originalText); inferred != "" {
		return &model.DistrictAutoFill{
			InferredDistrict: inferred,
			InferenceSource:  "original_text",
		}
	}

	// Last resort: call the geocoder to resolve the district via 高德地图 API.
	// This is the most reliable method for addresses where even the street name
	// is missing or ambiguous (e.g. "惠州市张屋山一巷二号" — only a lane number).
	// We geocode the deduplicated originalText (dedup happens in the caller layer
	// but we also dedup here as a safety net) to get the complete administrative hierarchy.
	if v.geocoder != nil && originalText != "" {
		ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		geocodeAddr := DeduplicateAdministrativePrefix(originalText)
		if geoResult := v.geocoder.Geocode(ctx, geocodeAddr); geoResult != nil {
			// Only accept the result if the city matches.
			if NormalizeCity(city) == NormalizeCity(geoResult.City) {
				return &model.DistrictAutoFill{
					InferredDistrict: geoResult.District,
					InferenceSource:  "geocoder",
				}
			}
		}
	}

	return nil
}

// inferFromStreet looks up the street→district map for the given city.
func (v *DistrictValidator) inferFromStreet(city, street string) string {
	if street == "" || city == "" {
		return ""
	}

	// Normalize: strip 街道/镇/乡/路/大道/巷 suffix.
	normalized := normalizeStreet(street)

	// Direct lookup.
	if d, ok := streetToDistrict[city+":"+normalized]; ok {
		return d
	}
	if d, ok := streetToDistrict[city+":"+street]; ok {
		return d
	}

	// Try matching street prefix (handles cases like "金湖社区" matching "金湖").
	for key, district := range streetToDistrict {
		if !strings.HasPrefix(key, city+":") {
			continue
		}
		streetKey := strings.TrimPrefix(key, city+":")
		if strings.Contains(normalized, streetKey) || strings.Contains(streetKey, normalized) {
			return district
		}
		// Check if street starts with the key.
		if strings.HasPrefix(normalized, streetKey) {
			return district
		}
		// Check if key starts with the street.
		if strings.HasPrefix(streetKey, normalized) {
			return district
		}
	}

	return ""
}

// inferFromDetail extracts a district hint from detail address fragments.
// It searches both district names (from cityDistricts) and street/community names
// (from streetToDistrict) to find a matching district.
func (v *DistrictValidator) inferFromDetail(city, detail string) string {
	if detail == "" || city == "" {
		return ""
	}

	// Strategy 1: look for known district names in the detail.
	for d := range cityDistricts[city] {
		if strings.Contains(detail, d) {
			return d
		}
		// Also try the abbreviated form (without suffix).
		if suffix := stripSuffix(d); suffix != "" && strings.Contains(detail, suffix) {
			return d
		}
	}

	// Strategy 2: look for known street/community names that map to a district.
	// This handles cases like "河南岸街道" → "惠城区".
	for key, district := range streetToDistrict {
		if !strings.HasPrefix(key, city+":") {
			continue
		}
		streetKey := strings.TrimPrefix(key, city+":")
		if strings.Contains(detail, streetKey) {
			return district
		}
		// Also try prefix match for partial street names in detail.
		if strings.HasPrefix(streetKey, detail) || strings.HasPrefix(detail, streetKey) {
			return district
		}
	}

	return ""
}

// inferFromOriginalText searches the full original address text for district/street hints.
// It is a last-resort fallback when the detail field is too narrow to contain the hint.
func (v *DistrictValidator) inferFromOriginalText(city, originalText string) string {
	if originalText == "" || city == "" {
		return ""
	}

	// Strategy 1: look for known district names in the original text.
	for d := range cityDistricts[city] {
		if strings.Contains(originalText, d) {
			return d
		}
	}

	// Strategy 2: look for known street/community names that map to a district.
	for key, district := range streetToDistrict {
		if !strings.HasPrefix(key, city+":") {
			continue
		}
		streetKey := strings.TrimPrefix(key, city+":")
		if strings.Contains(originalText, streetKey) {
			return district
		}
	}

	return ""
}

// findCorrectDistrict uses street name and known street→district mappings
// to find the correct district for the given city.
func (v *DistrictValidator) findCorrectDistrict(city, street, detail string) string {
	// Strategy 1: look up the street in the street→district map.
	if street != "" {
		if d := v.inferFromStreet(city, street); d != "" {
			return d
		}
	}

	// Strategy 2: look in the detail for district hints.
	if d := v.inferFromDetail(city, detail); d != "" {
		return d
	}

	return ""
}

// findCorrectDistrictByName checks whether the district name itself contains
// a hint about a different district. Returns the correct district or "".
func (v *DistrictValidator) findCorrectDistrictByName(city, district string) string {
	if city == "" || district == "" {
		return ""
	}

	// Check if the district name contains a known street/community name that maps to a district.
	for key, targetDistrict := range streetToDistrict {
		if !strings.HasPrefix(key, city+":") {
			continue
		}
		streetName := strings.TrimPrefix(key, city+":")
		// Does the captured district string contain the street name?
		if strings.Contains(district, streetName) {
			return targetDistrict
		}
	}
	return ""
}

// normalizeDistrict removes common suffixes from a district name.
func normalizeDistrict(d string) string {
	for _, suffix := range []string{"区", "县", "市辖区", "县级市", "新区"} {
		if strings.HasSuffix(d, suffix) {
			return d
		}
	}
	return d
}

// normalizeStreet removes common street suffixes.
func normalizeStreet(s string) string {
	// Strip trailing street/town/village suffixes.
	for _, suffix := range []string{"街道", "镇", "乡", "社区", "路", "大道", "巷", "村"} {
		if strings.HasSuffix(s, suffix) {
			s = strings.TrimSuffix(s, suffix)
		}
	}
	return s
}

// stripSuffix removes the trailing 区/县 suffix.
func stripSuffix(d string) string {
	for _, suffix := range []string{"区", "县"} {
		if strings.HasSuffix(d, suffix) {
			return strings.TrimSuffix(d, suffix)
		}
	}
	return d
}
