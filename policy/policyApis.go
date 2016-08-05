//
//Copyright [2016] [SnapRoute Inc]
//
//Licensed under the Apache License, Version 2.0 (the "License");
//you may not use this file except in compliance with the License.
//You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
//	 Unless required by applicable law or agreed to in writing, software
//	 distributed under the License is distributed on an "AS IS" BASIS,
//	 WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
//	 See the License for the specific language governing permissions and
//	 limitations under the License.
//
// _______  __       __________   ___      _______.____    __    ____  __  .___________.  ______  __    __
// |   ____||  |     |   ____\  \ /  /     /       |\   \  /  \  /   / |  | |           | /      ||  |  |  |
// |  |__   |  |     |  |__   \  V  /     |   (----` \   \/    \/   /  |  | `---|  |----`|  ,----'|  |__|  |
// |   __|  |  |     |   __|   >   <       \   \      \            /   |  |     |  |     |  |     |   __   |
// |  |     |  `----.|  |____ /  .  \  .----)   |      \    /\    /    |  |     |  |     |  `----.|  |  |  |
// |__|     |_______||_______/__/ \__\ |_______/        \__/  \__/     |__|     |__|      \______||__|  |__|
//

// policyApis.go
package policy

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"utils/netUtils"
	"utils/patriciaDB"
	"utils/policy/policyCommonDefs"
)

type PolicyStmt struct { //policy engine uses this
	Name            string
	Precedence      int
	MatchConditions string
	Conditions      []string
	Actions         []string
	PolicyList      []string
	LocalDBSliceIdx int8
	//	ImportStmt      bool
	//	ExportStmt      bool
	//	GlobalStmt      bool
}
type PolicyStmtConfig struct {
	Name            string
	AdminState      string
	MatchConditions string
	Conditions      []string
	Actions         []string
}

type Policy struct {
	Name                    string
	Precedence              int
	MatchType               string
	PolicyStmtPrecedenceMap map[int]string
	LocalDBSliceIdx         int8
	ImportPolicy            bool
	ExportPolicy            bool
	GlobalPolicy            bool
	PolicyType              string
	Extensions              interface{}
}

type PolicyDefinitionStmtPrecedence struct {
	Precedence int
	Statement  string
}
type PolicyDefinitionConfig struct {
	Name                       string
	Precedence                 int
	MatchType                  string
	PolicyDefinitionStatements []PolicyDefinitionStmtPrecedence
	Export                     bool
	Import                     bool
	Global                     bool
	PolicyType                 string
	Extensions                 interface{}
}

type PrefixPolicyListInfo struct {
	ipPrefix   patriciaDB.Prefix
	policyName string
	lowRange   int
	highRange  int
}

func validMatchConditions(matchConditionStr string) (valid bool) {
	if matchConditionStr == "any" || matchConditionStr == "all" {
		valid = true
	}
	return valid
}
func (db *PolicyEngineDB) UpdateProtocolPolicyTable(protoType string, name string, op int) {
	db.Logger.Info("updateProtocolPolicyTable for protocol ", protoType, " policy name ", name, " op ", op)
	var i int
	policyList := db.ProtocolPolicyListDB[protoType]
	if policyList == nil {
		if op == del {
			db.Logger.Info("Cannot find the policy map for this protocol, so cannot delete")
			return
		}
		policyList = make([]string, 0)
	}
	if op == add {
		policyList = append(policyList, name)
	}
	found := false
	if op == del {
		for i = 0; i < len(policyList); i++ {
			if policyList[i] == name {
				db.Logger.Info("Found the policy in the protocol policy table, deleting it")
				found = true
				break
			}
		}
		if found {
			policyList = append(policyList[:i], policyList[i+1:]...)
		}
	}
	db.ProtocolPolicyListDB[protoType] = policyList
}
func (db *PolicyEngineDB) UpdatePrefixPolicyTableWithPrefix(ipAddr string, name string, op int, lowRange int, highRange int) {
	db.Logger.Info("updatePrefixPolicyTableWithPrefix ", ipAddr)
	var i int
	ipPrefix, err := netUtils.GetNetworkPrefixFromCIDR(ipAddr)
	if err != nil {
		db.Logger.Err("ipPrefix invalid ")
		return
	}
	var policyList []PrefixPolicyListInfo
	var prefixPolicyListInfo PrefixPolicyListInfo
	policyListItem := db.PrefixPolicyListDB.Get(ipPrefix)
	if policyListItem != nil && reflect.TypeOf(policyListItem).Kind() != reflect.Slice {
		db.Logger.Err("Incorrect data type for this prefix ")
		return
	}
	if policyListItem == nil {
		if op == del {
			db.Logger.Err("Cannot find the policy map for this prefix, so cannot delete")
			return
		}
		policyList = make([]PrefixPolicyListInfo, 0)
	} else {
		policyListSlice := reflect.ValueOf(policyListItem)
		policyList = make([]PrefixPolicyListInfo, 0)
		for i = 0; i < policyListSlice.Len(); i++ {
			policyList = append(policyList, policyListSlice.Index(i).Interface().(PrefixPolicyListInfo))
		}
	}
	if op == add {
		prefixPolicyListInfo.ipPrefix = ipPrefix
		prefixPolicyListInfo.policyName = name
		prefixPolicyListInfo.lowRange = lowRange
		prefixPolicyListInfo.highRange = highRange
		policyList = append(policyList, prefixPolicyListInfo)
	}
	found := false
	if op == del {
		for i = 0; i < len(policyList); i++ {
			if policyList[i].policyName == name {
				db.Logger.Info("Found the policy in the prefix policy table, deleting it")
				break
			}
		}
		if found {
			policyList = append(policyList[:i], policyList[i+1:]...)
		}
	}
	db.PrefixPolicyListDB.Set(ipPrefix, policyList)
}
func (db *PolicyEngineDB) UpdatePrefixPolicyTableWithMaskRange(ipAddr string, masklength string, name string, op int) {
	db.Logger.Info("updatePrefixPolicyTableWithMaskRange")
	maskList := strings.Split(masklength, "-")
	if len(maskList) != 2 {
		db.Logger.Err("Invalid masklength range")
		return
	}
	lowRange, err := strconv.Atoi(maskList[0])
	if err != nil {
		db.Logger.Err("lowRange mask not valid")
		return
	}
	highRange, err := strconv.Atoi(maskList[1])
	if err != nil {
		db.Logger.Err("highRange mask not valid")
		return
	}
	db.Logger.Info("lowRange = ", lowRange, " highrange = ", highRange)
	db.UpdatePrefixPolicyTableWithPrefix(ipAddr, name, op, lowRange, highRange)
	/*for idx := lowRange; idx < highRange; idx++ {
		ipMask := net.CIDRMask(idx, 32)
		ipMaskStr := (net.IP(ipMask)).String()
		db.Logger.Info("idx ", idx, "ipMaskStr = ", ipMaskStr)
		ipPrefix, err := netutils.GetNetowrkPrefixFromStrings(ipAddr, ipMaskStr)
		if err != nil {
			db.Logger.Info("Invalid prefix")
			return
		}
		updatePrefixPolicyTableWithPrefix(ipPrefix, name, op, lowRange, highRange)
	}*/
}
func (db *PolicyEngineDB) UpdatePrefixPolicyTableWithPrefixSet(prefixSet string, name string, op int) {
	db.Logger.Info("updatePrefixPolicyTableWithPrefixSet")
}
func (db *PolicyEngineDB) UpdatePrefixPolicyTable(conditionInfo interface{}, name string, op int) {
	condition := conditionInfo.(MatchPrefixConditionInfo)
	db.Logger.Info("updatePrefixPolicyTable for prefixSet ", condition.PrefixSet, " prefix ", condition.Prefix, " policy name ", name, " op ", op)
	if condition.UsePrefixSet {
		db.Logger.Info("Need to look up Prefix set to get the prefixes")
		db.UpdatePrefixPolicyTableWithPrefixSet(condition.PrefixSet, name, op)
	} else {
		if condition.Prefix.MasklengthRange == "exact" {
			/*			ipPrefix, err := netutils.GetNetworkPrefixFromCIDR(condition.Prefix.IpPrefix)
						if err != nil {
							db.Logger.Info("ipPrefix invalid ")
							return
						}*/
			db.UpdatePrefixPolicyTableWithPrefix(condition.Prefix.IpPrefix, name, op, -1, -1)
		} else {
			db.Logger.Info("Masklength= ", condition.Prefix.MasklengthRange)
			db.UpdatePrefixPolicyTableWithMaskRange(condition.Prefix.IpPrefix, condition.Prefix.MasklengthRange, name, op)
		}
	}
}
func (db *PolicyEngineDB) UpdateStatements(policy Policy, stmt PolicyStmt, op int) (err error) {
	db.Logger.Info("UpdateStatements for stmt ", stmt.Name)
	var i int
	if stmt.PolicyList == nil {
		if op == del {
			db.Logger.Info("stmt.PolicyList nil")
			return err
		}
		stmt.PolicyList = make([]string, 0)
	}
	if op == add {
		stmt.PolicyList = append(stmt.PolicyList, policy.Name)
	}
	found := false
	if op == del {
		for i = 0; i < len(stmt.PolicyList); i++ {
			if stmt.PolicyList[i] == policy.Name {
				db.Logger.Info("Found the policy in the policy stmt table, deleting it")
				found = true
				break
			}
		}
		if found {
			stmt.PolicyList = append(stmt.PolicyList[:i], stmt.PolicyList[i+1:]...)
		}
	}
	db.PolicyStmtDB.Set(patriciaDB.Prefix(stmt.Name), stmt)
	return err
}

func (db *PolicyEngineDB) UpdateGlobalStatementTable(policy string, stmt string, op int) (err error) {
	db.Logger.Info("updateGlobalStatementTablestmt ", stmt, " with policy ", policy)
	var i int
	policyList := db.PolicyStmtPolicyMapDB[stmt]
	if policyList == nil {
		if op == del {
			db.Logger.Info("Cannot find the policy map for this stmt, so cannot delete")
			err = errors.New("Cannot find the policy map for this stmt, so cannot delete")
			return err
		}
		policyList = make([]string, 0)
	}
	if op == add {
		policyList = append(policyList, policy)
	}
	found := false
	if op == del {
		for i = 0; i < len(policyList); i++ {
			if policyList[i] == policy {
				db.Logger.Info("Found the policy in the policy stmt table, deleting it")
				found = true
				break
			}
		}
		if found {
			policyList = append(policyList[:i], policyList[i+1:]...)
		}
	}
	db.PolicyStmtPolicyMapDB[stmt] = policyList
	return err
}
func (db *PolicyEngineDB) UpdateConditions(policyStmt PolicyStmt, conditionName string, op int) (err error) {
	db.Logger.Info("updateConditions for condition ", conditionName)
	var i int
	conditionItem := db.PolicyConditionsDB.Get(patriciaDB.Prefix(conditionName))
	if conditionItem == nil {
		db.Logger.Info("Condition name ", conditionName, " not defined")
		err = errors.New("Condition name not defined")
		return err
	}
	condition := conditionItem.(PolicyCondition)
	switch condition.ConditionType {
	case policyCommonDefs.PolicyConditionTypeProtocolMatch:
		db.Logger.Info("PolicyConditionTypeProtocolMatch")
		db.UpdateProtocolPolicyTable(condition.ConditionInfo.(string), policyStmt.Name, op)
		break
	case policyCommonDefs.PolicyConditionTypeDstIpPrefixMatch:
		db.Logger.Info("PolicyConditionTypeDstIpPrefixMatch")
		db.UpdatePrefixPolicyTable(condition.ConditionInfo, policyStmt.Name, op)
		break
	}
	if condition.PolicyStmtList == nil {
		if op == del {
			db.Logger.Info("condition.PolicyStmtList empty")
			err = errors.New("condition.PolicyStmtList Empty")
			return err
		}
		condition.PolicyStmtList = make([]string, 0)
	}
	if op == add {
		condition.PolicyStmtList = append(condition.PolicyStmtList, policyStmt.Name)
	}
	found := false
	if op == del {
		for i = 0; i < len(condition.PolicyStmtList); i++ {
			if condition.PolicyStmtList[i] == policyStmt.Name {
				db.Logger.Info("Found the policyStmt in the condition's list, deleting it")
				found = true
				break
			}
		}
		if found {
			condition.PolicyStmtList = append(condition.PolicyStmtList[:i], condition.PolicyStmtList[i+1:]...)
		}
	}
	db.PolicyConditionsDB.Set(patriciaDB.Prefix(conditionName), condition)
	return err
}

func (db *PolicyEngineDB) UpdateActions(policyStmt PolicyStmt, action PolicyAction, op int) (err error) {
	db.Logger.Info("updateActions for action ", action.Name)
	var i int
	if action.PolicyStmtList == nil {
		if op == del {
			db.Logger.Info("action.PolicyStmtList empty")
			err = errors.New("action.PolicyStmtLisy Empty")
			return err
		}
		action.PolicyStmtList = make([]string, 0)
	}
	if op == add {
		action.PolicyStmtList = append(action.PolicyStmtList, policyStmt.Name)
	}
	found := false
	if op == del {
		for i = 0; i < len(action.PolicyStmtList); i++ {
			if action.PolicyStmtList[i] == policyStmt.Name {
				db.Logger.Info("Found the policyStmt in the action's list, deleting it")
				found = true
				break
			}
		}
		if found {
			action.PolicyStmtList = append(action.PolicyStmtList[:i], action.PolicyStmtList[i+1:]...)
		}
	}

	db.PolicyActionsDB.Set(patriciaDB.Prefix(action.Name), action)
	return err
}
func (db *PolicyEngineDB) ValidatePolicyStatementCreate(cfg PolicyStmtConfig) (err error) {
	db.Logger.Info("ValidatePolicyStatementCreate")
	policyStmt := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.Name))
	if policyStmt != nil {
		db.Logger.Err("Duplicate Policy definition name")
		err = errors.New("Duplicate policy definition")
		return err
	}
	if !validMatchConditions(cfg.MatchConditions) {
		db.Logger.Err("Invalid match conditions - try any/all")
		err = errors.New("Invalid match conditions - try any/all")
		return err
	}
	if len(cfg.Actions) > 1 {
		db.Logger.Err("Cannot have more than 1 action in a policy")
		err = errors.New("Cannot have more than 1 action in a policy")
		return err
	}
	if cfg.Actions[0] != "permit" && cfg.Actions[0] != "deny" {
		db.Logger.Err("Invalid stmt actions, can only be one of permit/deny")
		return errors.New("Invalid stmt actions")
	}
	i := 0
	for i = 0; i < len(cfg.Conditions); i++ {
		if db.PolicyConditionsDB.Get(patriciaDB.Prefix(cfg.Conditions[i])) == nil {
			db.Logger.Err("Condition ", cfg.Conditions[i], " not found ")
			return errors.New("Condition not found")
		}
	}
	return err
}

func (db *PolicyEngineDB) CreatePolicyStatement(cfg PolicyStmtConfig) (err error) {
	db.Logger.Info("CreatePolicyStatement")
	policyStmt := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.Name))
	var i int
	if policyStmt == nil {
		db.Logger.Info("Defining a new policy statement with name ", cfg.Name)
		var newPolicyStmt PolicyStmt
		newPolicyStmt.Name = cfg.Name
		newPolicyStmt.MatchConditions = cfg.MatchConditions
		if len(cfg.Conditions) > 0 {
			db.Logger.Info("Policy Statement has ", len(cfg.Conditions), " number of conditions")
			newPolicyStmt.Conditions = make([]string, 0)
			for i = 0; i < len(cfg.Conditions); i++ {
				newPolicyStmt.Conditions = append(newPolicyStmt.Conditions, cfg.Conditions[i])
				err = db.UpdateConditions(newPolicyStmt, cfg.Conditions[i], add)
				if err != nil {
					db.Logger.Info("updateConditions returned err ", err)
					err = errors.New("Error with updateConditions")
					return err
				}
			}
		}
		if len(cfg.Actions) > 0 {
			db.Logger.Info("Policy Statement has ", len(cfg.Actions), " number of actions")
			if len(cfg.Actions) > 1 {
				db.Logger.Err("Cannot have more than 1 action in a policy")
				err = errors.New("Cannot have more than 1 action in a policy")
				return err
			}
			newPolicyStmt.Actions = make([]string, 0)
			newPolicyStmt.Actions = append(newPolicyStmt.Actions, cfg.Actions[0])
		}
		newPolicyStmt.LocalDBSliceIdx = int8(len(*db.LocalPolicyStmtDB))
		if ok := db.PolicyStmtDB.Insert(patriciaDB.Prefix(cfg.Name), newPolicyStmt); ok != true {
			db.Logger.Err(" return value not ok")
			err = errors.New("error inserting into policy stmt DB")
			return err
		}
		db.LocalPolicyStmtDB.updateLocalDB(patriciaDB.Prefix(cfg.Name), add)
	} else {
		db.Logger.Err("Duplicate Policy definition name")
		err = errors.New("Duplicate policy definition")
		return err
	}
	return err
}

func (db *PolicyEngineDB) ValidatePolicyStatementDelete(cfg PolicyStmtConfig) (err error) {
	db.Logger.Err("ValidatePolicyStatementCreate")
	ok := db.PolicyStmtDB.Match(patriciaDB.Prefix(cfg.Name))
	if !ok {
		err = errors.New("No policy statement with this name found")
		return err
	}
	policyStmtInfoGet := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.Name))
	if policyStmtInfoGet != nil {
		policyStmtInfo := policyStmtInfoGet.(PolicyStmt)
		if len(policyStmtInfo.PolicyList) != 0 {
			db.Logger.Err("This policy stmt is being used by one or more policies. Delete the policies before deleting the stmt")
			err = errors.New("This policy stmt is being used by one or more policies. Delete the policies before deleting the stmt")
			return err
		}
	}
	return nil
}
func (db *PolicyEngineDB) DeletePolicyStatement(cfg PolicyStmtConfig) (err error) {
	db.Logger.Info("DeletePolicyStatement for name ", cfg.Name)
	ok := db.PolicyStmtDB.Match(patriciaDB.Prefix(cfg.Name))
	if !ok {
		err = errors.New("No policy statement with this name found")
		return err
	}
	policyStmtInfoGet := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.Name))
	if policyStmtInfoGet != nil {
		policyStmtInfo := policyStmtInfoGet.(PolicyStmt)
		if len(policyStmtInfo.PolicyList) != 0 {
			db.Logger.Err("This policy stmt is being used by one or more policies. Delete the policies before deleting the stmt")
			err = errors.New("This policy stmt is being used by one or more policies. Delete the policies before deleting the stmt")
			return err
		}
		//invalidate localPolicyStmt
		/*	   if policyStmtInfo.LocalDBSliceIdx < int8(len(*db.LocalPolicyStmtDB)) {
		          db.Logger.Info("local DB slice index for this policy stmt is ", policyStmtInfo.LocalDBSliceIdx)
				  LocalPolicyStmtDB := LocalDBSlice (*db.LocalPolicyStmtDB)
				  LocalPolicyStmtDB[policyStmtInfo.LocalDBSliceIdx].IsValid = false
			   }*/
		// PolicyEngineTraverseAndReverse(policyStmtInfo)
		db.Logger.Info("Deleting policy statement with name ", cfg.Name)
		if ok := db.PolicyStmtDB.Delete(patriciaDB.Prefix(cfg.Name)); ok != true {
			db.Logger.Err(" return value not ok for delete PolicyStmtDB")
			err = errors.New("error with delteing policy stmt")
			return err
		}
		db.LocalPolicyStmtDB.updateLocalDB(patriciaDB.Prefix(cfg.Name), del)
		//update other tables
		if len(policyStmtInfo.Conditions) > 0 {
			for i := 0; i < len(policyStmtInfo.Conditions); i++ {
				db.UpdateConditions(policyStmtInfo, policyStmtInfo.Conditions[i], del)
			}
		}
		/*		if len(policyStmtInfo.Actions) > 0 {
				var action PolicyAction
				for i := 0; i < len(policyStmtInfo.Actions); i++ {
					actionItem := db.PolicyActionsDB.Get(patriciaDB.Prefix(policyStmtInfo.Actions[i]))
					if actionItem != nil {
						action = actionItem.(PolicyAction)
					} else {
						db.Logger.Err("action name ", policyStmtInfo.Actions[i], " not defined")
						err = errors.New("action name not defined")
					}
					db.UpdateActions(policyStmtInfo, action, del)
				}
			}*/
	}
	return err
}
func (db *PolicyEngineDB) UpdateApplyPolicy(info ApplyPolicyInfo, apply bool) {
	db.Logger.Info("ApplyPolicy")
	applyPolicy := info.ApplyPolicy
	action := info.Action
	conditions := make([]string, 0)
	for i := 0; i < len(info.Conditions); i++ {
		conditions = append(conditions, info.Conditions[i])
	}
	exportType, importType, _ := db.PolicyActionType(action.ActionType)
	db.Logger.Info("exportType:", exportType, " importType:", importType)
	if importType {
		db.Logger.Info("Adding ", applyPolicy.Name, " as import policy")
		if db.ImportPolicyPrecedenceMap == nil {
			db.ImportPolicyPrecedenceMap = make(map[int]string)
		}
		db.ImportPolicyPrecedenceMap[int(applyPolicy.Precedence)] = applyPolicy.Name
	} else if exportType {
		db.Logger.Info("Adding ", applyPolicy.Name, " as export policy")
		if db.ExportPolicyPrecedenceMap == nil {
			db.ExportPolicyPrecedenceMap = make(map[int]string)
		}
		db.ExportPolicyPrecedenceMap[int(applyPolicy.Precedence)] = applyPolicy.Name
	}
	if db.ApplyPolicyMap[applyPolicy.Name] == nil {
		db.ApplyPolicyMap[applyPolicy.Name] = make([]ApplyPolicyInfo, 0)
	}
	if HasActionInfo(db.ApplyPolicyMap[applyPolicy.Name], action) {
		//for now do nothing, need to handle on update of conditions/stmt/policy
	} else {
		db.ApplyPolicyMap[applyPolicy.Name] = append(db.ApplyPolicyMap[applyPolicy.Name], ApplyPolicyInfo{applyPolicy, action, conditions})
	}
	if apply {
		db.PolicyEngineTraverseAndApplyPolicy(info)
	}
}
func (db *PolicyEngineDB) ValidatePolicyDefinitionCreate(cfg PolicyDefinitionConfig) (err error) {
	db.Logger.Err("ValidatePolicyDefinitionCreate")
	policy := db.PolicyDB.Get(patriciaDB.Prefix(cfg.Name))
	if policy != nil {
		db.Logger.Err("Duplicate Policy definition name")
		err = errors.New("Duplicate policy definition")
		return err
	}
	var newPolicy Policy
	newPolicy.Name = cfg.Name
	newPolicy.Precedence = cfg.Precedence
	newPolicy.MatchType = cfg.MatchType
	for i := 0; i < len(cfg.PolicyDefinitionStatements); i++ {
		Item := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.PolicyDefinitionStatements[i].Statement))
		if Item == nil {
			db.Logger.Info("stmt name ", cfg.PolicyDefinitionStatements[i].Statement, " not defined")
			err = errors.New("stmt name not defined")
			return err
		}
		stmt := Item.(PolicyStmt)
		for cds := 0; cds < len(stmt.Actions); cds++ {
			if !db.ConditionCheckForPolicyType(stmt.Conditions[cds], cfg.PolicyType) {
				db.Logger.Err("Trying to add statement with incompatible condition ", stmt.Conditions[cds], " to this policy of policyType: ", cfg.PolicyType)
				return errors.New("Incompatible condition type ")
			}
		}
		//TO_DO: similar validation for actions/sub-actions
	}
	return err
}
func (db *PolicyEngineDB) CreatePolicyDefinition(cfg PolicyDefinitionConfig) (err error) {
	db.Logger.Info("CreatePolicyDefinition")
	policy := db.PolicyDB.Get(patriciaDB.Prefix(cfg.Name))
	var i int
	if policy == nil {
		db.Logger.Info("Defining a new policy with name ", cfg.Name)
		var newPolicy Policy
		newPolicy.Name = cfg.Name
		newPolicy.Precedence = cfg.Precedence
		newPolicy.MatchType = cfg.MatchType
		db.Logger.Info("Policy has ", len(cfg.PolicyDefinitionStatements), " number of statements")
		newPolicy.PolicyStmtPrecedenceMap = make(map[int]string)
		for i = 0; i < len(cfg.PolicyDefinitionStatements); i++ {
			var stmt PolicyStmt
			db.Logger.Info("Adding statement ", cfg.PolicyDefinitionStatements[i].Statement, " at precedence id ", cfg.PolicyDefinitionStatements[i].Precedence)
			if newPolicy.PolicyStmtPrecedenceMap[int(cfg.PolicyDefinitionStatements[i].Precedence)] != "" {
				db.Logger.Info(" Cannot add multiple statements at the same priority level during create")
				//undo the statement mappings for the statements already added to this policy
				for idx := 0; idx < i; idx++ {
					Item := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.PolicyDefinitionStatements[idx].Statement))
					if Item != nil {
						stmt = Item.(PolicyStmt)
						err = db.UpdateStatements(newPolicy, stmt, del)
						if err != nil {
							db.Logger.Info("updateStatements returned err ", err)
							err = errors.New("error with updateStatements")
						}
					} else {
						db.Logger.Err("Statement ", cfg.PolicyDefinitionStatements[idx].Statement, " not defined")
						err = errors.New("stmt name not defined")
					}
					err = db.UpdateGlobalStatementTable(newPolicy.Name, cfg.PolicyDefinitionStatements[idx].Statement, del)
					if err != nil {
						db.Logger.Info("UpdateGlobalStatementTable returned err ", err)
						err = errors.New("Error with UpdateGlobalStatementTable")
					}
				}
				return errors.New(" Cannot add multiple statements at the same priority level during create")
			}
			newPolicy.PolicyStmtPrecedenceMap[int(cfg.PolicyDefinitionStatements[i].Precedence)] = cfg.PolicyDefinitionStatements[i].Statement
			Item := db.PolicyStmtDB.Get(patriciaDB.Prefix(cfg.PolicyDefinitionStatements[i].Statement))
			if Item != nil {
				stmt = Item.(PolicyStmt)
				err = db.UpdateStatements(newPolicy, stmt, add)
				if err != nil {
					db.Logger.Info("updateStatements returned err ", err)
					err = errors.New("error with updateStatements")
				}
			} else {
				db.Logger.Err("Statement ", cfg.PolicyDefinitionStatements[i].Statement, " not defined")
				err = errors.New("stmt name not defined")
			}
			err = db.UpdateGlobalStatementTable(newPolicy.Name, cfg.PolicyDefinitionStatements[i].Statement, add)
			if err != nil {
				db.Logger.Info("UpdateGlobalStatementTable returned err ", err)
				err = errors.New("Error with UpdateGlobalStatementTable")
			}
		}
		newPolicy.LocalDBSliceIdx = int8(len(*db.LocalPolicyDB))
		newPolicy.Extensions = cfg.Extensions
		if ok := db.PolicyDB.Insert(patriciaDB.Prefix(cfg.Name), newPolicy); ok != true {
			db.Logger.Info(" return value not ok")
			err = errors.New("error inserting into policyDB")
			return err
		}
		db.LocalPolicyDB.updateLocalDB(patriciaDB.Prefix(cfg.Name), add)
	} else {
		db.Logger.Err("Duplicate Policy definition name")
		err = errors.New("Duplicate policy definition")
		return err
	}
	return err
}
func (db *PolicyEngineDB) ValidatePolicyDefinitionDelete(cfg PolicyDefinitionConfig) (err error) {
	db.Logger.Info("ValidatePolicyDefinitionDelete")
	policyItem := db.PolicyDB.Get(patriciaDB.Prefix(cfg.Name))
	if policyItem == nil {
		db.Logger.Err("Policy not defined")
		err = errors.New("Policy not defined")
		return err
	}
	policy := policyItem.(Policy)
	if db.ApplyPolicyMap[policy.Name] != nil {
		db.Logger.Err(" Policy being applied, cannot delete it")
		err = errors.New(fmt.Sprintln("Policy being used, cannot delete"))
		return err
	}
	return err
}
func (db *PolicyEngineDB) DeletePolicyDefinition(cfg PolicyDefinitionConfig) (err error) {
	db.Logger.Info("DeletePolicyDefinition for name ", cfg.Name)
	ok := db.PolicyDB.Match(patriciaDB.Prefix(cfg.Name))
	if !ok {
		err = errors.New("No policy with this name found")
		return err
	}
	policyInfoGet := db.PolicyDB.Get(patriciaDB.Prefix(cfg.Name))
	if policyInfoGet != nil {
		policyInfo := policyInfoGet.(Policy)
		db.PolicyEngineTraverseAndReversePolicy(policyInfo)
		db.Logger.Info("Deleting policy with name ", cfg.Name)
		if ok := db.PolicyDB.Delete(patriciaDB.Prefix(cfg.Name)); ok != true {
			db.Logger.Err(" return value not ok for delete PolicyDB")
			err = errors.New("error deleting from policyDB")
			return err
		}
		db.LocalPolicyDB.updateLocalDB(patriciaDB.Prefix(cfg.Name), del)
		var stmt PolicyStmt
		for _, v := range policyInfo.PolicyStmtPrecedenceMap {
			err = db.UpdateGlobalStatementTable(policyInfo.Name, v, del)
			if err != nil {
				db.Logger.Info("UpdateGlobalStatementTable returned err ", err)
				err = errors.New("UpdateGlobalStatementTable returned err")
			}
			Item := db.PolicyStmtDB.Get(patriciaDB.Prefix(v))
			if Item != nil {
				stmt = Item.(PolicyStmt)
				err = db.UpdateStatements(policyInfo, stmt, del)
				if err != nil {
					db.Logger.Info("updateStatements returned err ", err)
					err = errors.New("UpdateStatements returned err")
				}
			} else {
				db.Logger.Err("Statement ", v, " not defined")
				err = errors.New("statement name not defined")
			}
		}
		if policyInfo.ExportPolicy {
			if db.ExportPolicyPrecedenceMap != nil {
				delete(db.ExportPolicyPrecedenceMap, int(policyInfo.Precedence))
			}
		}
		if policyInfo.ImportPolicy {
			if db.ImportPolicyPrecedenceMap != nil {
				delete(db.ImportPolicyPrecedenceMap, int(policyInfo.Precedence))
			}
		}
	}
	return err
}
